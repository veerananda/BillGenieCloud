package services

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

const smtpTimeout = 12 * time.Second

func smtpEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// smtpEnvelopeAddress returns the bare email for SMTP MAIL FROM.
// Display names like `BillGenie <ops@example.com>` are valid in headers but
// not as the envelope sender.
func smtpEnvelopeAddress(from string) (string, error) {
	from = strings.TrimSpace(from)
	if from == "" {
		return "", errors.New("empty from address")
	}
	addr, err := mail.ParseAddress(from)
	if err != nil {
		return "", fmt.Errorf("invalid SMTP from address %q: %w", from, err)
	}
	if addr.Address == "" {
		return "", fmt.Errorf("invalid SMTP from address %q", from)
	}
	return addr.Address, nil
}

// smtpLoginAuth implements the LOGIN SASL mechanism used by many providers
// (including GoDaddy) that prefer LOGIN over PLAIN.
type smtpLoginAuth struct {
	username, password string
	step               int
}

func (a *smtpLoginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if server == nil {
		return "", nil, errors.New("missing smtp server info")
	}
	if !server.TLS {
		return "", nil, errors.New("unencrypted connection")
	}
	a.step = 0
	return "LOGIN", nil, nil
}

func (a *smtpLoginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(strings.TrimSpace(string(fromServer)))
	a.step++
	switch {
	case strings.Contains(prompt, "username") || a.step == 1:
		return []byte(a.username), nil
	case strings.Contains(prompt, "password") || a.step >= 2:
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected smtp LOGIN challenge %q", string(fromServer))
	}
}

type smtpSettings struct {
	Host       string
	Port       string
	User       string
	Pass       string
	FromHeader string
	FromAddr   string
}

func loadSMTPSettings() (*smtpSettings, error) {
	host := smtpEnv("SMTP_HOST")
	port := smtpEnv("SMTP_PORT")
	user := smtpEnv("SMTP_USER", "SMTP_MAIL")
	pass := smtpEnv("SMTP_PASS", "SMTP_APP_PASSWORD")
	fromHeader := smtpEnv("SMTP_FROM")
	if fromHeader == "" {
		fromHeader = user
	}

	if host == "" || port == "" || user == "" || pass == "" {
		return nil, errors.New("smtp is not configured (SMTP_HOST/SMTP_PORT and SMTP_USER/SMTP_PASS or SMTP_MAIL/SMTP_APP_PASSWORD)")
	}

	userAddr, err := smtpEnvelopeAddress(user)
	if err != nil {
		return nil, fmt.Errorf("SMTP_USER/SMTP_MAIL: %w", err)
	}

	fromAddr, err := smtpEnvelopeAddress(fromHeader)
	if err != nil {
		return nil, err
	}

	// GoDaddy and similar hosts often reject or drop mail when MAIL FROM
	// is not the authenticated mailbox.
	if !strings.EqualFold(fromAddr, userAddr) {
		fromAddr = userAddr
		fromHeader = fmt.Sprintf("BillGenie <%s>", userAddr)
	}

	return &smtpSettings{
		Host:       host,
		Port:       port,
		User:       user,
		Pass:       pass,
		FromHeader: fromHeader,
		FromAddr:   fromAddr,
	}, nil
}

// SMTPConfigSummary returns non-secret SMTP settings for ops debugging.
func SMTPConfigSummary() map[string]string {
	cfg, err := loadSMTPSettings()
	if err != nil {
		return map[string]string{"configured": "false", "error": err.Error()}
	}
	return map[string]string{
		"configured": "true",
		"host":       cfg.Host,
		"port":       cfg.Port,
		"user":       cfg.User,
		"from":       cfg.FromHeader,
	}
}

// ProbeSMTPDial verifies the process can open an SMTP session and authenticate.
func ProbeSMTPDial() error {
	cfg, err := loadSMTPSettings()
	if err != nil {
		return err
	}
	client, conn, err := smtpConnect(cfg)
	if err != nil {
		return fmt.Errorf("%w (host=%s port=%s user=%s)", err, cfg.Host, cfg.Port, cfg.User)
	}
	defer conn.Close()
	defer client.Close()
	if err := smtpAuthenticate(client, cfg); err != nil {
		return fmt.Errorf("%w (host=%s port=%s user=%s)", err, cfg.Host, cfg.Port, cfg.User)
	}
	_ = client.Quit()
	return nil
}

// SendSMTPTestEmail sends a short ops test message to confirm end-to-end delivery.
func SendSMTPTestEmail(to string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("to email is required")
	}
	return sendEmailSMTP(
		to,
		"BillGenie SMTP test",
		"This is a test email from the BillGenie platform SMTP probe.\n\nIf you received this, outbound mail is working.\n",
	)
}

// sendEmailSMTP sends an email using SMTP settings from environment variables.
// Port 465 uses implicit TLS. Other ports require STARTTLS.
func sendEmailSMTP(to, subject, body string) error {
	cfg, err := loadSMTPSettings()
	if err != nil {
		return err
	}

	msg := []byte(
		"To: " + to + "\r\n" +
			"From: " + cfg.FromHeader + "\r\n" +
			"Subject: " + subject + "\r\n" +
			"MIME-Version: 1.0\r\n" +
			"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
			"\r\n" +
			body,
	)

	client, conn, err := smtpConnect(cfg)
	if err != nil {
		return fmt.Errorf("%w (host=%s port=%s user=%s)", err, cfg.Host, cfg.Port, cfg.User)
	}
	defer conn.Close()
	defer client.Close()

	if err := smtpAuthenticate(client, cfg); err != nil {
		return fmt.Errorf("%w (host=%s port=%s user=%s)", err, cfg.Host, cfg.Port, cfg.User)
	}
	if err := client.Mail(cfg.FromAddr); err != nil {
		return fmt.Errorf("smtp mail from %s: %w", cfg.FromAddr, err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt %s: %w", to, err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data close: %w", err)
	}
	_ = client.Quit()
	return nil
}

func smtpConnect(cfg *smtpSettings) (*smtp.Client, net.Conn, error) {
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	tlsConfig := &tls.Config{ServerName: cfg.Host}
	dialer := &net.Dialer{Timeout: smtpTimeout}

	var (
		conn net.Conn
		err  error
	)
	if cfg.Port == "465" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp tls dial %s: %w", addr, err)
		}
	} else {
		conn, err = dialer.Dial("tcp", addr)
		if err != nil {
			return nil, nil, fmt.Errorf("smtp dial %s: %w", addr, err)
		}
	}

	if err := conn.SetDeadline(time.Now().Add(smtpTimeout)); err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("smtp deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, fmt.Errorf("smtp client: %w", err)
	}

	if cfg.Port != "465" {
		ok, _ := client.Extension("STARTTLS")
		if !ok {
			_ = client.Close()
			_ = conn.Close()
			return nil, nil, fmt.Errorf("smtp server %s does not advertise STARTTLS on port %s (try SMTP_PORT=465)", cfg.Host, cfg.Port)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			_ = conn.Close()
			return nil, nil, fmt.Errorf("smtp starttls: %w", err)
		}
	}

	return client, conn, nil
}

func smtpAuthenticate(client *smtp.Client, cfg *smtpSettings) error {
	supports, mechanisms := client.Extension("AUTH")
	mech := strings.ToUpper(mechanisms)

	useLogin := !supports || strings.Contains(mech, "LOGIN")
	usePlain := !supports || strings.Contains(mech, "PLAIN")

	// Prefer LOGIN for GoDaddy / workspace hosts; fall back to PLAIN on a fresh
	// connection is hard mid-session, so pick one based on advertisement.
	if useLogin && (!usePlain || strings.Contains(strings.ToLower(cfg.Host), "secureserver") || strings.Contains(strings.ToLower(cfg.Host), "godaddy") || strings.Contains(strings.ToLower(cfg.Host), "titan")) {
		if err := client.Auth(&smtpLoginAuth{username: cfg.User, password: cfg.Pass}); err != nil {
			return fmt.Errorf("smtp auth LOGIN: %w", err)
		}
		return nil
	}
	if usePlain {
		if err := client.Auth(smtp.PlainAuth("", cfg.User, cfg.Pass, cfg.Host)); err != nil {
			return fmt.Errorf("smtp auth PLAIN: %w", err)
		}
		return nil
	}
	if useLogin {
		if err := client.Auth(&smtpLoginAuth{username: cfg.User, password: cfg.Pass}); err != nil {
			return fmt.Errorf("smtp auth LOGIN: %w", err)
		}
		return nil
	}
	return fmt.Errorf("smtp auth: no supported AUTH mechanisms (server: %q)", mechanisms)
}
