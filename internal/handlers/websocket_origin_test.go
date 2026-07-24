package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOriginMatchesRequestHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://billgenie-api.fly.dev/ws", nil)
	req.Host = "billgenie-api.fly.dev"
	if !originMatchesRequestHost(req, "https://billgenie-api.fly.dev") {
		t.Fatal("expected RN Origin matching API host to be allowed")
	}
	if originMatchesRequestHost(req, "https://evil.example") {
		t.Fatal("unexpected allow for foreign origin")
	}
}

func TestOriginMatchesForwardedHost(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8080/ws", nil)
	req.Host = "127.0.0.1:8080"
	req.Header.Set("X-Forwarded-Host", "billgenie-api.fly.dev")
	if !originMatchesRequestHost(req, "https://billgenie-api.fly.dev") {
		t.Fatal("expected Origin to match X-Forwarded-Host")
	}
}
