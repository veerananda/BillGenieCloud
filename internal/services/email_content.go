package services

import (
	"fmt"
	"html"
	"os"
	"strings"
)

const (
	billGenieWebsiteURL = "https://thebillgenie.com"
	defaultSupportEmail = "hello@thebillgenie.com"
)

type composedEmail struct {
	Subject string
	Text    string
	HTML    string
}

// supportEmail is the public contact address shown in transactional mail.
func supportEmail() string {
	if v := strings.TrimSpace(os.Getenv("SUPPORT_EMAIL")); v != "" {
		return v
	}
	if v := smtpEnv("RESEND_FROM", "SMTP_FROM"); v != "" {
		if addr, err := smtpEnvelopeAddress(v); err == nil && addr != "" {
			return addr
		}
	}
	return defaultSupportEmail
}

func emailSenderDisplayName() string {
	return "BillGenie"
}

func emailGreeting(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Hi,"
	}
	return fmt.Sprintf("Hi %s,", name)
}

func transactionalEmailFooterText() string {
	return strings.Join([]string{
		"—",
		emailSenderDisplayName(),
		fmt.Sprintf("Restaurant billing & operations · %s", billGenieWebsiteURL),
		fmt.Sprintf("Questions? Reply to this email or write to %s", supportEmail()),
		"This message was sent by BillGenie on behalf of your restaurant account.",
	}, "\n")
}

func transactionalEmailFooterHTML() string {
	site := html.EscapeString(billGenieWebsiteURL)
	support := html.EscapeString(supportEmail())
	return fmt.Sprintf(`<hr style="border:none;border-top:1px solid #e5e7eb;margin:28px 0 16px;" />
<p style="margin:0 0 6px;font-size:14px;line-height:1.5;color:#111827;"><strong>%s</strong></p>
<p style="margin:0 0 6px;font-size:13px;line-height:1.5;color:#6b7280;">Restaurant billing &amp; operations · <a href="%s" style="color:#0b6e4f;">%s</a></p>
<p style="margin:0 0 6px;font-size:13px;line-height:1.5;color:#6b7280;">Questions? Reply to this email or write to <a href="mailto:%s" style="color:#0b6e4f;">%s</a></p>
<p style="margin:0;font-size:12px;line-height:1.5;color:#9ca3af;">This message was sent by BillGenie on behalf of your restaurant account.</p>`,
		html.EscapeString(emailSenderDisplayName()), site, site, support, support)
}

func wrapTransactionalHTML(title, bodyHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8" /><meta name="viewport" content="width=device-width, initial-scale=1" /><title>%s</title></head>
<body style="margin:0;padding:0;background:#f3f4f6;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f3f4f6;padding:24px 12px;">
    <tr><td align="center">
      <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:560px;background:#ffffff;border-radius:12px;padding:28px 24px;border:1px solid #e5e7eb;">
        <tr><td>
          <p style="margin:0 0 20px;font-size:18px;font-weight:700;color:#0b6e4f;">%s</p>
          %s
          %s
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, html.EscapeString(title), html.EscapeString(emailSenderDisplayName()), bodyHTML, transactionalEmailFooterHTML())
}

func buildVerificationEmail(ownerName, restaurantName, verificationLink string) composedEmail {
	greeting := emailGreeting(ownerName)
	restaurantName = strings.TrimSpace(restaurantName)

	thanksText := "Thanks for registering with BillGenie."
	thanksHTML := "Thanks for registering with BillGenie."
	if restaurantName != "" {
		thanksText = fmt.Sprintf("Thanks for registering with BillGenie for %s.", restaurantName)
		thanksHTML = fmt.Sprintf("Thanks for registering with BillGenie for <strong>%s</strong>.", html.EscapeString(restaurantName))
	}

	subject := "Verify your email for BillGenie"

	text := fmt.Sprintf(`%s

%s

Please confirm this email address so we can finish setting up your account:

%s

What happens next
1. Open the link above (it expires in 24 hours).
2. Wait for BillGenie to approve your restaurant.
3. We'll email you again when you can sign in.

If you did not create a BillGenie account, you can ignore this message.

%s`,
		greeting,
		thanksText,
		verificationLink,
		transactionalEmailFooterText(),
	)

	button := fmt.Sprintf(
		`<p style="margin:24px 0;"><a href="%s" style="display:inline-block;background:#0b6e4f;color:#ffffff;text-decoration:none;padding:12px 20px;border-radius:8px;font-size:14px;font-weight:600;">Verify email address</a></p>`,
		html.EscapeString(verificationLink),
	)

	bodyHTML := fmt.Sprintf(`<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#111827;">%s</p>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#111827;">%s</p>
<p style="margin:0 0 8px;font-size:15px;line-height:1.6;color:#111827;">Please confirm this email address so we can finish setting up your account.</p>
%s
<p style="margin:0 0 8px;font-size:13px;line-height:1.6;color:#6b7280;">Or copy this link into your browser:</p>
<p style="margin:0 0 20px;font-size:13px;line-height:1.6;word-break:break-all;"><a href="%s" style="color:#0b6e4f;">%s</a></p>
<p style="margin:0 0 8px;font-size:15px;font-weight:600;color:#111827;">What happens next</p>
<ol style="margin:0 0 16px;padding-left:20px;font-size:14px;line-height:1.7;color:#374151;">
  <li>Open the verification link (expires in 24 hours).</li>
  <li>Wait for BillGenie to approve your restaurant.</li>
  <li>We'll email you again when you can sign in.</li>
</ol>
<p style="margin:0;font-size:13px;line-height:1.6;color:#6b7280;">If you did not create a BillGenie account, you can ignore this message.</p>`,
		html.EscapeString(greeting),
		thanksHTML,
		button,
		html.EscapeString(verificationLink),
		html.EscapeString(verificationLink),
	)

	return composedEmail{
		Subject: subject,
		Text:    text,
		HTML:    wrapTransactionalHTML(subject, bodyHTML),
	}
}

func buildApprovalEmail(ownerName, restaurantName, loginNumber string) composedEmail {
	greeting := emailGreeting(ownerName)
	name := strings.TrimSpace(restaurantName)
	if name == "" {
		name = "your restaurant"
	}
	loginNumber = strings.TrimSpace(loginNumber)

	subject := "You're approved — welcome to BillGenie"

	loginBlockText := "Open the BillGenie app or website and sign in with the admin login number you received at registration."
	if loginNumber != "" {
		loginBlockText = fmt.Sprintf("Sign in with your admin login number:\n\n%s\n\nUse the BillGenie app or website to get started.", loginNumber)
	}

	text := fmt.Sprintf(`%s

Great news — %s has been reviewed and approved by the BillGenie team.

Your email is verified and your account is active. You can sign in now.

%s

Need help getting set up? Reply to this email or contact %s — we're happy to help.

%s`,
		greeting,
		name,
		loginBlockText,
		supportEmail(),
		transactionalEmailFooterText(),
	)

	loginHTML := `<p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#111827;">Open the BillGenie app or website and sign in with the admin login number you received at registration.</p>`
	if loginNumber != "" {
		loginHTML = fmt.Sprintf(`<p style="margin:0 0 8px;font-size:15px;line-height:1.6;color:#111827;">Sign in with your admin login number:</p>
<p style="margin:0 0 8px;font-size:22px;font-weight:700;letter-spacing:0.04em;color:#0b6e4f;">%s</p>
<p style="margin:0 0 16px;font-size:14px;line-height:1.6;color:#6b7280;">Use the BillGenie app or website to get started.</p>`,
			html.EscapeString(loginNumber))
	}

	bodyHTML := fmt.Sprintf(`<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#111827;">%s</p>
<p style="margin:0 0 12px;font-size:15px;line-height:1.6;color:#111827;">Great news — <strong>%s</strong> has been reviewed and approved by the BillGenie team.</p>
<p style="margin:0 0 16px;font-size:15px;line-height:1.6;color:#111827;">Your email is verified and your account is active. You can sign in now.</p>
%s
<p style="margin:0;font-size:14px;line-height:1.6;color:#374151;">Need help getting set up? Reply to this email or contact <a href="mailto:%s" style="color:#0b6e4f;">%s</a> — we're happy to help.</p>`,
		html.EscapeString(greeting),
		html.EscapeString(name),
		loginHTML,
		html.EscapeString(supportEmail()),
		html.EscapeString(supportEmail()),
	)

	return composedEmail{
		Subject: subject,
		Text:    text,
		HTML:    wrapTransactionalHTML(subject, bodyHTML),
	}
}
