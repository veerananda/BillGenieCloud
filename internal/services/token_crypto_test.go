package services

import "testing"

func TestHashSecretDeterministic(t *testing.T) {
	a := hashSecret("token-value")
	b := hashSecret("token-value")
	if a != b {
		t.Fatalf("expected deterministic hash")
	}
	if len(a) != 64 {
		t.Fatalf("expected sha256 hex length 64, got %d", len(a))
	}
	if a == "token-value" {
		t.Fatalf("hash must not equal plaintext")
	}
}

func TestSessionTokenMatchesDualRead(t *testing.T) {
	raw := "eyJhbGciOiJIUzI1NiJ9.payload.sig"
	if !sessionTokenMatches(raw, raw) {
		t.Fatal("plaintext dual-read should match")
	}
	if !sessionTokenMatches(hashSecret(raw), raw) {
		t.Fatal("hashed storage should match raw token")
	}
	if sessionTokenMatches(hashSecret(raw), "other") {
		t.Fatal("should not match different token")
	}
}

func TestGenerateNumericOTPLength(t *testing.T) {
	otp, err := generateNumericOTP(6)
	if err != nil {
		t.Fatal(err)
	}
	if len(otp) != 6 {
		t.Fatalf("expected 6 digits, got %q", otp)
	}
	for _, c := range otp {
		if c < '0' || c > '9' {
			t.Fatalf("non-digit in otp: %q", otp)
		}
	}
}
