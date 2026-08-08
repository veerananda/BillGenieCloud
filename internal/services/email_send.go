package services

import "fmt"

// sendEmail sends transactional email.
// Prefers Resend HTTPS API when RESEND_API_KEY is set (works on DigitalOcean);
// otherwise falls back to SMTP_* (blocked on many cloud providers).
func sendEmail(to, subject, body string) error {
	if smtpEnv("RESEND_API_KEY") != "" {
		if err := sendEmailResend(to, subject, body); err != nil {
			return fmt.Errorf("resend: %w", err)
		}
		return nil
	}
	return sendEmailSMTP(to, subject, body)
}
