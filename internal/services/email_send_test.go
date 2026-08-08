package services

import (
	"strings"
	"testing"
)

func TestSendEmailResendRequiresAPIKey(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "")
	err := sendEmailResend("user@example.com", composedEmail{Subject: "Subject", Text: "Body"})
	if err == nil || !strings.Contains(err.Error(), "RESEND_API_KEY") {
		t.Fatalf("expected missing API key error, got %v", err)
	}
}

func TestSendEmailResendRejectsEmptyRecipient(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("RESEND_FROM", "BillGenie <hello@thebillgenie.com>")
	err := sendEmailResend("  ", composedEmail{Subject: "Subject", Text: "Body"})
	if err == nil || !strings.Contains(err.Error(), "recipient") {
		t.Fatalf("expected empty recipient error, got %v", err)
	}
}

func TestSendEmailRoutesToResendWhenKeySet(t *testing.T) {
	t.Setenv("RESEND_API_KEY", "re_test")
	t.Setenv("RESEND_FROM", "not-an-email")
	err := sendEmail("user@example.com", "Subject", "Body")
	if err == nil || !strings.Contains(err.Error(), "resend") {
		t.Fatalf("expected resend route error, got %v", err)
	}
}
