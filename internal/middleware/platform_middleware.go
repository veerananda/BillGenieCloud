package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

type platformKeyEntry struct {
	actor     string
	keyDigest [32]byte
}

// PlatformAuthMiddleware protects BillGenie creator-only ops routes.
//
// Auth sources (first match wins):
//  1. PLATFORM_OPS_API_KEYS — comma-separated actor=secret pairs (preferred; actor bound to key)
//  2. PLATFORM_OPS_API_KEY  — legacy shared secret; optional X-Platform-Actor header
//
// Optional PLATFORM_OPS_IP_ALLOWLIST — comma-separated IPs or CIDRs (empty = allow all).
func PlatformAuthMiddleware() gin.HandlerFunc {
	entries := loadPlatformKeyEntries()
	allowlist := parseIPAllowlist(os.Getenv("PLATFORM_OPS_IP_ALLOWLIST"))

	return func(c *gin.Context) {
		if len(entries) == 0 {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "platform ops API is not configured",
			})
			c.Abort()
			return
		}

		if len(allowlist) > 0 && !clientIPAllowed(c, allowlist) {
			log.Printf("❌ Platform ops IP rejected: %s", c.ClientIP())
			c.JSON(http.StatusForbidden, gin.H{"error": "platform ops access denied"})
			c.Abort()
			return
		}

		presented := extractPlatformAPIKey(c)
		if presented == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid platform credentials"})
			c.Abort()
			return
		}

		actor, ok := matchPlatformKey(presented, entries)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid platform credentials"})
			c.Abort()
			return
		}

		// When using the legacy shared key, allow an explicit actor label from the header.
		if actor == "platform_ops" {
			if hdr := strings.TrimSpace(c.GetHeader("X-Platform-Actor")); hdr != "" {
				actor = hdr
			}
		}

		c.Set("platform_actor", actor)
		c.Set("platform_client_ip", c.ClientIP())
		c.Next()
	}
}

func extractPlatformAPIKey(c *gin.Context) string {
	key := strings.TrimSpace(c.GetHeader("X-Platform-Api-Key"))
	if key != "" {
		return key
	}
	auth := strings.TrimSpace(c.GetHeader("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func loadPlatformKeyEntries() []platformKeyEntry {
	var out []platformKeyEntry

	multi := strings.TrimSpace(os.Getenv("PLATFORM_OPS_API_KEYS"))
	if multi != "" {
		for _, part := range strings.Split(multi, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			actor, secret, found := strings.Cut(part, "=")
			actor = strings.TrimSpace(actor)
			secret = strings.TrimSpace(secret)
			if !found || actor == "" || secret == "" {
				log.Printf("⚠️  Skipping malformed PLATFORM_OPS_API_KEYS entry")
				continue
			}
			out = append(out, platformKeyEntry{
				actor:     actor,
				keyDigest: sha256.Sum256([]byte(secret)),
			})
		}
	}

	legacy := strings.TrimSpace(os.Getenv("PLATFORM_OPS_API_KEY"))
	if legacy != "" {
		out = append(out, platformKeyEntry{
			actor:     "platform_ops",
			keyDigest: sha256.Sum256([]byte(legacy)),
		})
	}

	return out
}

func matchPlatformKey(presented string, entries []platformKeyEntry) (actor string, ok bool) {
	presentedDigest := sha256.Sum256([]byte(presented))
	for _, e := range entries {
		if subtle.ConstantTimeCompare(presentedDigest[:], e.keyDigest[:]) == 1 {
			return e.actor, true
		}
	}
	return "", false
}

type ipRule struct {
	ip    net.IP
	cidr  *net.IPNet
	isNet bool
}

func parseIPAllowlist(raw string) []ipRule {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var rules []ipRule
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			_, network, err := net.ParseCIDR(part)
			if err != nil {
				log.Printf("⚠️  Invalid PLATFORM_OPS_IP_ALLOWLIST CIDR %q: %v", part, err)
				continue
			}
			rules = append(rules, ipRule{cidr: network, isNet: true})
			continue
		}
		ip := net.ParseIP(part)
		if ip == nil {
			log.Printf("⚠️  Invalid PLATFORM_OPS_IP_ALLOWLIST IP %q", part)
			continue
		}
		rules = append(rules, ipRule{ip: ip})
	}
	return rules
}

func clientIPAllowed(c *gin.Context, rules []ipRule) bool {
	ipStr := strings.TrimSpace(c.ClientIP())
	// Prefer first X-Forwarded-For hop when Fly/proxy sets it (Gin ClientIP already handles this
	// when TrustedProxies are configured; fall back to RemoteAddr host).
	if fwd := strings.TrimSpace(c.GetHeader("X-Forwarded-For")); fwd != "" {
		ipStr = strings.TrimSpace(strings.Split(fwd, ",")[0])
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	for _, rule := range rules {
		if rule.isNet {
			if rule.cidr.Contains(ip) {
				return true
			}
			continue
		}
		if rule.ip.Equal(ip) {
			return true
		}
	}
	return false
}

// DigestPlatformKey is exported for tests.
func DigestPlatformKey(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}
