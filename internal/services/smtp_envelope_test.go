package services

import "testing"

func TestSMTPEnvelopeAddress(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{in: "ops@example.com", want: "ops@example.com"},
		{in: "BillGenie <ops@example.com>", want: "ops@example.com"},
		{in: `"BillGenie" <ops@example.com>`, want: "ops@example.com"},
		{in: "  ops@example.com  ", want: "ops@example.com"},
		{in: "BillGenie ops@example.com", wantErr: true},
		{in: "", wantErr: true},
	}
	for _, tt := range tests {
		got, err := smtpEnvelopeAddress(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("%q: expected error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("%q: got %q want %q", tt.in, got, tt.want)
		}
	}
}
