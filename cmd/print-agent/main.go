// Print agent: listens for BillGenieCloud SSE wake events, claims queued KOT/bill
// jobs, and prints over LAN ESC/POS (TCP :9100) or Bluetooth-classic printers
// exposed as serial ports (Windows COM after OS pairing, or /dev/cu.* on macOS/Linux).
//
// Usage:
//
//	set BILLGENIE_API_URL=https://api.thebillgenie.com
//	set BILLGENIE_PRINT_AGENT_KEY=bgpa_...
//	go run ./cmd/print-agent
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"go.bug.st/serial"
)

type claimResponse struct {
	Jobs []printJob `json:"jobs"`
}

type printJob struct {
	ID              string `json:"id"`
	JobType         string `json:"job_type"`
	Target          string `json:"target"`
	PayloadText     string `json:"payload_text"`
	PrinterHost     string `json:"printer_host"`
	PrinterPort     int    `json:"printer_port"`
	TopFeedLines    int    `json:"top_feed_lines"`
	BottomFeedLines int    `json:"bottom_feed_lines"`
}

var (
	comPortRe   = regexp.MustCompile(`(?i)^COM\d+$`)
	serialDevRe = regexp.MustCompile(`(?i)^(/dev/|\\\\\.\\)`)
)

func main() {
	apiURL := strings.TrimRight(os.Getenv("BILLGENIE_API_URL"), "/")
	agentKey := strings.TrimSpace(os.Getenv("BILLGENIE_PRINT_AGENT_KEY"))
	agentID := os.Getenv("BILLGENIE_PRINT_AGENT_ID")
	if agentID == "" {
		hostname, _ := os.Hostname()
		agentID = hostname
	}
	if apiURL == "" || agentKey == "" {
		log.Fatal("Set BILLGENIE_API_URL and BILLGENIE_PRINT_AGENT_KEY")
	}

	log.Printf("Print agent starting → %s (agent_id=%s)", apiURL, agentID)
	log.Printf("Supports TCP (LAN/Wi-Fi) and serial/Bluetooth COM ports (e.g. COM5, serial:COM5)")
	log.Printf("Mode: event-driven SSE (no polling)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	wake := make(chan struct{}, 1)
	go func() {
		backoff := time.Second
		for {
			select {
			case <-stop:
				return
			default:
			}
			err := listenEvents(apiURL, agentKey, wake, stop)
			if err != nil {
				log.Printf("SSE error: %v — reconnecting in %s", err, backoff)
			}
			select {
			case <-stop:
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
			}
		}
	}()

	// Catch any jobs queued before SSE connected.
	signalWake(wake)

	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}
	pending := false

	for {
		select {
		case <-stop:
			log.Println("Shutting down")
			return
		case <-wake:
			pending = true
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(150 * time.Millisecond)
		case <-debounce.C:
			if !pending {
				continue
			}
			pending = false
			if err := claimAndPrint(apiURL, agentKey, agentID); err != nil {
				log.Printf("claim error: %v", err)
			}
		}
	}
}

func signalWake(wake chan struct{}) {
	select {
	case wake <- struct{}{}:
	default:
	}
}

func listenEvents(apiURL, agentKey string, wake chan struct{}, stop <-chan os.Signal) error {
	req, err := http.NewRequest(http.MethodGet, apiURL+"/print-agent/events", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Print-Agent-Key", agentKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("events %s: %s", resp.Status, string(raw))
	}

	log.Printf("SSE connected to %s/print-agent/events", apiURL)
	signalWake(wake)

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-stop:
			return nil
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return fmt.Errorf("SSE stream closed")
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			continue
		}
		// Wake on named jobs events or generic data frames from the API.
		if strings.HasPrefix(line, "event: jobs") || strings.HasPrefix(line, "data:") {
			signalWake(wake)
		}
	}
}

func claimAndPrint(apiURL, agentKey, agentID string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"agent_id": agentID,
		"limit":    5,
	})
	req, err := http.NewRequest(http.MethodPost, apiURL+"/print-agent/jobs/claim", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Print-Agent-Key", agentKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("claim %s: %s", resp.Status, string(raw))
	}

	var claimed claimResponse
	if err := json.Unmarshal(raw, &claimed); err != nil {
		return err
	}
	for _, job := range claimed.Jobs {
		target := describePrintTarget(job.PrinterHost, job.PrinterPort)
		log.Printf("printing %s job %s → %s", job.JobType, job.ID, target)
		if err := printESCPOS(job.PrinterHost, job.PrinterPort, job.PayloadText, job.TopFeedLines, job.BottomFeedLines); err != nil {
			log.Printf("print failed: %v", err)
			_ = reportJob(apiURL, agentKey, job.ID, true, err.Error())
			continue
		}
		_ = reportJob(apiURL, agentKey, job.ID, false, "")
	}
	// If the batch was full, drain remaining jobs immediately.
	if len(claimed.Jobs) >= 5 {
		return claimAndPrint(apiURL, agentKey, agentID)
	}
	return nil
}

func reportJob(apiURL, agentKey, jobID string, failed bool, errMsg string) error {
	path := "/print-agent/jobs/" + jobID + "/complete"
	var body io.Reader
	if failed {
		path = "/print-agent/jobs/" + jobID + "/fail"
		b, _ := json.Marshal(map[string]string{"error": errMsg})
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, apiURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("X-Print-Agent-Key", agentKey)
	if failed {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func describePrintTarget(host string, port int) string {
	if portName, ok := serialPortName(host); ok {
		return "serial:" + portName
	}
	if port <= 0 {
		port = 9100
	}
	return fmt.Sprintf("%s:%d", host, port)
}

// serialPortName returns a serial device path when host is a Bluetooth/serial target.
// Accepted forms: COM5, serial:COM5, bt:COM5, /dev/cu.Printer, \\.\COM5
func serialPortName(host string) (string, bool) {
	h := strings.TrimSpace(host)
	if h == "" {
		return "", false
	}
	lower := strings.ToLower(h)
	for _, prefix := range []string{"serial:", "bt:", "bluetooth:"} {
		if strings.HasPrefix(lower, prefix) {
			h = strings.TrimSpace(h[len(prefix):])
			lower = strings.ToLower(h)
			break
		}
	}
	if comPortRe.MatchString(h) {
		return strings.ToUpper(h), true
	}
	if serialDevRe.MatchString(h) {
		return h, true
	}
	return "", false
}

func clampFeed(n int) int {
	if n < 0 {
		return 0
	}
	if n > 20 {
		return 20
	}
	return n
}

// expandBillGenieQRMarkers replaces <<<BILLGENIE_QR>>>...<<<END_QR>>> blocks with ESC/POS QR bytes.
func expandBillGenieQRMarkers(text string) []byte {
	const start = "<<<BILLGENIE_QR>>>"
	const end = "<<<END_QR>>>"
	var out bytes.Buffer
	rest := text
	for {
		i := strings.Index(rest, start)
		if i < 0 {
			out.WriteString(rest)
			break
		}
		out.WriteString(rest[:i])
		rest = rest[i+len(start):]
		j := strings.Index(rest, end)
		if j < 0 {
			out.WriteString(start)
			out.WriteString(rest)
			break
		}
		payload := strings.TrimSpace(rest[:j])
		rest = rest[j+len(end):]
		if payload != "" {
			out.Write(buildESCPOSQRCode(payload, 5))
			out.WriteByte('\n')
		}
	}
	return out.Bytes()
}

// buildESCPOSQRCode encodes data as an Epson-compatible QR symbol (model 2).
func buildESCPOSQRCode(data string, moduleSize byte) []byte {
	if moduleSize < 1 {
		moduleSize = 1
	}
	if moduleSize > 16 {
		moduleSize = 16
	}
	payload := []byte(data)
	var buf bytes.Buffer
	// GS ( k — select model 2
	buf.Write([]byte{0x1d, 0x28, 0x6b, 0x04, 0x00, 0x31, 0x41, 0x32, 0x00})
	// module size
	buf.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x43, moduleSize})
	// error correction M (49)
	buf.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x45, 0x31})
	// store data
	storeLen := len(payload) + 3
	buf.Write([]byte{0x1d, 0x28, 0x6b, byte(storeLen & 0xff), byte((storeLen >> 8) & 0xff), 0x31, 0x50, 0x30})
	buf.Write(payload)
	// print
	buf.Write([]byte{0x1d, 0x28, 0x6b, 0x03, 0x00, 0x31, 0x51, 0x30})
	return buf.Bytes()
}

func buildESCPOSPayload(text string, topFeed, bottomFeed int) []byte {
	topFeed = clampFeed(topFeed)
	bottomFeed = clampFeed(bottomFeed)
	var buf bytes.Buffer
	buf.Write([]byte{0x1b, 0x40}) // ESC @ init
	if topFeed > 0 {
		buf.WriteString(strings.Repeat("\n", topFeed))
	}
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	body := expandBillGenieQRMarkers(normalized)
	buf.Write(body)
	if len(body) == 0 || body[len(body)-1] != '\n' {
		buf.WriteByte('\n')
	}
	// Advance paper past the print head before cutting (thermal head sits above cutter).
	if bottomFeed > 0 {
		buf.WriteString(strings.Repeat("\n", bottomFeed))
	}
	// GS V 1 = partial cut — safer on many 58mm models than full cut (GS V 0).
	buf.Write([]byte{0x1d, 0x56, 0x01})
	return buf.Bytes()
}

// printESCPOS sends plain text with init + cut to a LAN or serial/Bluetooth printer.
func printESCPOS(host string, port int, text string, topFeed, bottomFeed int) error {
	if host == "" {
		return fmt.Errorf("empty printer host")
	}
	payload := buildESCPOSPayload(text, topFeed, bottomFeed)

	if portName, ok := serialPortName(host); ok {
		return printESCPOSSerial(portName, payload)
	}

	if port <= 0 {
		port = 9100
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
	_, err = conn.Write(payload)
	return err
}

func printESCPOSSerial(portName string, payload []byte) error {
	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(portName, mode)
	if err != nil {
		// Some BT SPP printers prefer 115200.
		mode.BaudRate = 115200
		port, err = serial.Open(portName, mode)
		if err != nil {
			return fmt.Errorf("open serial %s: %w (pair the Bluetooth printer in Windows and use its COM port)", portName, err)
		}
	}
	defer port.Close()
	_ = port.SetReadTimeout(2 * time.Second)

	n, err := port.Write(payload)
	if err != nil {
		return fmt.Errorf("write serial %s: %w", portName, err)
	}
	if n < len(payload) {
		return fmt.Errorf("short write to %s: %d/%d bytes", portName, n, len(payload))
	}
	// Give the printer a moment to finish before closing the RFCOMM link.
	time.Sleep(300 * time.Millisecond)
	return nil
}
