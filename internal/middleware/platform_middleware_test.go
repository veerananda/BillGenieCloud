package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPlatformAuthMultiKeyBindsActor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PLATFORM_OPS_API_KEY", "")
	t.Setenv("PLATFORM_OPS_API_KEYS", "veera=super-secret-a,mani=super-secret-b")
	t.Setenv("PLATFORM_OPS_IP_ALLOWLIST", "")

	r := gin.New()
	r.GET("/platform/ping", PlatformAuthMiddleware(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"actor": c.GetString("platform_actor")})
	})

	req := httptest.NewRequest(http.MethodGet, "/platform/ping", nil)
	req.Header.Set("X-Platform-Api-Key", "super-secret-b")
	req.Header.Set("X-Platform-Actor", "attacker")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"actor":"mani"`) {
		t.Fatalf("expected actor mani, got %s", w.Body.String())
	}
}

func TestPlatformAuthRejectsBadKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PLATFORM_OPS_API_KEYS", "veera=super-secret-a")
	t.Setenv("PLATFORM_OPS_API_KEY", "")
	t.Setenv("PLATFORM_OPS_IP_ALLOWLIST", "")

	r := gin.New()
	r.GET("/platform/ping", PlatformAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/platform/ping", nil)
	req.Header.Set("X-Platform-Api-Key", "wrong")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestPlatformAuthIPAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("PLATFORM_OPS_API_KEYS", "veera=super-secret-a")
	t.Setenv("PLATFORM_OPS_API_KEY", "")
	t.Setenv("PLATFORM_OPS_IP_ALLOWLIST", "203.0.113.10")

	r := gin.New()
	r.GET("/platform/ping", PlatformAuthMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/platform/ping", nil)
	req.Header.Set("X-Platform-Api-Key", "super-secret-a")
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed IP, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/platform/ping", nil)
	req2.Header.Set("X-Platform-Api-Key", "super-secret-a")
	req2.Header.Set("X-Forwarded-For", "203.0.113.10")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for allowlisted IP, got %d", w2.Code)
	}
}

func TestParseIPAllowlistCIDR(t *testing.T) {
	rules := parseIPAllowlist("10.0.0.0/8,192.168.1.5")
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}
