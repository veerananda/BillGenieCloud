package services

import (
	"strings"
	"testing"
)

func TestBuildVerificationEmailIncludesSenderAndSteps(t *testing.T) {
	mail := buildVerificationEmail("Priya", "Cafe Nila", "https://app.example/verify-email?token=abc")
	if !strings.Contains(mail.Subject, "Verify") {
		t.Fatalf("subject=%q", mail.Subject)
	}
	for _, needle := range []string{"Priya", "Cafe Nila", "https://app.example/verify-email?token=abc", "hello@thebillgenie.com", "thebillgenie.com", "What happens next"} {
		if !strings.Contains(mail.Text, needle) {
			t.Fatalf("text missing %q:\n%s", needle, mail.Text)
		}
	}
	if !strings.Contains(mail.HTML, "Verify email address") || !strings.Contains(mail.HTML, "BillGenie") {
		t.Fatalf("html missing expected content:\n%s", mail.HTML)
	}
}

func TestBuildApprovalEmailHighlightsLogin(t *testing.T) {
	mail := buildApprovalEmail("Ravi", "Spice Route", "BG-12345")
	if !strings.Contains(mail.Subject, "approved") {
		t.Fatalf("subject=%q", mail.Subject)
	}
	for _, needle := range []string{"Ravi", "Spice Route", "BG-12345", "hello@thebillgenie.com", "BillGenie"} {
		if !strings.Contains(mail.Text, needle) {
			t.Fatalf("text missing %q:\n%s", needle, mail.Text)
		}
	}
	if !strings.Contains(mail.HTML, "BG-12345") {
		t.Fatalf("html missing login number")
	}
}
