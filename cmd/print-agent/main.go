// Print agent: polls BillGenieCloud for queued KOT/bill jobs and prints over LAN ESC/POS (TCP :9100).
//
// Usage:
//
//	set BILLGENIE_API_URL=https://billgenie-api.fly.dev
//	set BILLGENIE_PRINT_AGENT_KEY=bgpa_...
//	go run ./cmd/print-agent
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

type claimResponse struct {
	Jobs []printJob `json:"jobs"`
}

type printJob struct {
	ID          string `json:"id"`
	JobType     string `json:"job_type"`
	Target      string `json:"target"`
	PayloadText string `json:"payload_text"`
	PrinterHost string `json:"printer_host"`
	PrinterPort int    `json:"printer_port"`
}

func main() {
	apiURL := strings.TrimRight(os.Getenv("BILLGENIE_API_URL"), "/")
	agentKey := strings.TrimSpace(os.Getenv("BILLGENIE_PRINT_AGENT_KEY"))
	agentID := os.Getenv("BILLGENIE_PRINT_AGENT_ID")
	if agentID == "" {
		hostname, _ := os.Hostname()
		agentID = hostname
	}
	pollSeconds := 2
	if apiURL == "" || agentKey == "" {
		log.Fatal("Set BILLGENIE_API_URL and BILLGENIE_PRINT_AGENT_KEY")
	}

	log.Printf("Print agent starting → %s (agent_id=%s)", apiURL, agentID)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(time.Duration(pollSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			log.Println("Shutting down")
			return
		case <-ticker.C:
			if err := pollOnce(apiURL, agentKey, agentID); err != nil {
				log.Printf("poll error: %v", err)
			}
		}
	}
}

func pollOnce(apiURL, agentKey, agentID string) error {
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
		log.Printf("printing %s job %s → %s:%d", job.JobType, job.ID, job.PrinterHost, job.PrinterPort)
		if err := printESCPOS(job.PrinterHost, job.PrinterPort, job.PayloadText); err != nil {
			log.Printf("print failed: %v", err)
			_ = reportJob(apiURL, agentKey, job.ID, true, err.Error())
			continue
		}
		_ = reportJob(apiURL, agentKey, job.ID, false, "")
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

// printESCPOS sends plain text with init + cut to a LAN thermal printer.
func printESCPOS(host string, port int, text string) error {
	if host == "" {
		return fmt.Errorf("empty printer host")
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

	var buf bytes.Buffer
	buf.Write([]byte{0x1b, 0x40}) // ESC @ init
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	buf.WriteString(normalized)
	if !strings.HasSuffix(normalized, "\n") {
		buf.WriteByte('\n')
	}
	buf.Write([]byte{0x1d, 0x56, 0x00}) // GS V 0 full cut

	_, err = conn.Write(buf.Bytes())
	return err
}
