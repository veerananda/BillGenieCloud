package services

import "fmt"

// sendEmail sends transactional plain-text email.
// Prefers Resend HTTPS API when RESEND_API_KEY is set (works on DigitalOcean);
// otherwise falls back to SMTP_* (blocked on many cloud providers).
func sendEmail(to, subject, body string) error {
	return sendComposedEmail(to, composedEmail{Subject: subject, Text: body})
}

func sendComposedEmail(to string, mail composedEmail) error {
	if smtpEnv("RESEND_API_KEY") != "" {
		if err := sendEmailResend(to, mail); err != nil {
			return fmt.Errorf("resend: %w", err)
		}
		return nil
	}
	return sendEmailSMTP(to, mail)
}
