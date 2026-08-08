package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const resendEmailsURL = "https://api.resend.com/emails"

type resendSendRequest struct {
	From    string   `json:"from"`
	To      []string `json:"to"`
	Subject string   `json:"subject"`
	Text    string   `json:"text"`
	HTML    string   `json:"html,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
}

type resendErrorBody struct {
	StatusCode int    `json:"statusCode"`
	Name       string `json:"name"`
	Message    string `json:"message"`
}

// sendEmailResend sends mail via Resend's HTTPS API (port 443).
// Env: RESEND_API_KEY (required), RESEND_FROM or SMTP_FROM (optional From header).
func sendEmailResend(to string, mail composedEmail) error {
	apiKey := smtpEnv("RESEND_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("resend is not configured (RESEND_API_KEY)")
	}

	from := smtpEnv("RESEND_FROM", "SMTP_FROM")
	if from == "" {
		from = fmt.Sprintf("%s <%s>", emailSenderDisplayName(), defaultSupportEmail)
	}
	if _, err := smtpEnvelopeAddress(from); err != nil {
		return fmt.Errorf("resend from address: %w", err)
	}

	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("resend recipient is empty")
	}

	subject := strings.TrimSpace(mail.Subject)
	text := mail.Text
	if text == "" && mail.HTML != "" {
		text = "Please view this email in an HTML-capable client."
	}

	payload, err := json.Marshal(resendSendRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		Text:    text,
		HTML:    mail.HTML,
		ReplyTo: supportEmail(),
	})
	if err != nil {
		return fmt.Errorf("resend marshal: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, resendEmailsURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend dial: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr resendErrorBody
		_ = json.Unmarshal(respBody, &apiErr)
		msg := strings.TrimSpace(apiErr.Message)
		if msg == "" {
			msg = strings.TrimSpace(string(respBody))
		}
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("resend api %d: %s", resp.StatusCode, msg)
	}
	return nil
}
