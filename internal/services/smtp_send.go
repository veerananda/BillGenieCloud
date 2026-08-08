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

func smtpEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return strings.Trim(v, `"'`)
		}
	}
	return ""
}

// smtpEnvelopeAddress returns the bare email for SMTP MAIL FROM.
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

// sendEmailSMTP sends email using SMTP_* env vars (fallback when RESEND_API_KEY is unset).
// Port 465 uses implicit TLS; other ports use STARTTLS + PLAIN auth.
// Dial/IO are time-bounded so a blocked SMTP port cannot hang HTTP handlers.
func sendEmailSMTP(to string, mail composedEmail) error {
	host := smtpEnv("SMTP_HOST")
	port := smtpEnv("SMTP_PORT")
	user := smtpEnv("SMTP_USER", "SMTP_MAIL")
	pass := smtpEnv("SMTP_PASS", "SMTP_APP_PASSWORD")
	fromHeader := smtpEnv("SMTP_FROM")
	if fromHeader == "" {
		fromHeader = user
	}

	if host == "" || port == "" || user == "" || pass == "" {
		return errors.New("smtp is not configured (SMTP_HOST/SMTP_PORT and SMTP_USER/SMTP_PASS or SMTP_MAIL/SMTP_APP_PASSWORD)")
	}

	fromAddr, err := smtpEnvelopeAddress(fromHeader)
	if err != nil {
		return err
	}

	subject := strings.TrimSpace(mail.Subject)
	text := mail.Text
	if text == "" && mail.HTML != "" {
		text = "Please view this email in an HTML-capable client."
	}

	var msg []byte
	replyTo := supportEmail()
	if strings.TrimSpace(mail.HTML) != "" {
		boundary := "billgenie_alt_boundary"
		var b strings.Builder
		b.WriteString("To: " + to + "\r\n")
		b.WriteString("From: " + fromHeader + "\r\n")
		b.WriteString("Reply-To: " + replyTo + "\r\n")
		b.WriteString("Subject: " + subject + "\r\n")
		b.WriteString("MIME-Version: 1.0\r\n")
		b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n\r\n")
		b.WriteString(text)
		b.WriteString("\r\n--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n\r\n")
		b.WriteString(mail.HTML)
		b.WriteString("\r\n--" + boundary + "--\r\n")
		msg = []byte(b.String())
	} else {
		msg = []byte(
			"To: " + to + "\r\n" +
				"From: " + fromHeader + "\r\n" +
				"Reply-To: " + replyTo + "\r\n" +
				"Subject: " + subject + "\r\n" +
				"MIME-Version: 1.0\r\n" +
				"Content-Type: text/plain; charset=\"utf-8\"\r\n" +
				"\r\n" +
				text,
		)
	}

	const timeout = 12 * time.Second
	addr := net.JoinHostPort(host, port)
	auth := smtp.PlainAuth("", user, pass, host)
	tlsConfig := &tls.Config{ServerName: host}
	dialer := &net.Dialer{Timeout: timeout}

	var conn net.Conn
	if port == "465" {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", addr, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	if port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(fromAddr); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
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
