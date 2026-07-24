package handlers

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const refreshCookieName = "bg_refresh"

func refreshCookieSecure() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SERVER_ENV")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	}
	return env == "production" || env == "prod"
}

func setRefreshTokenCookie(c *gin.Context, token string, maxAgeSeconds int) {
	if token == "" || maxAgeSeconds <= 0 {
		clearRefreshTokenCookie(c)
		return
	}
	secure := refreshCookieSecure()
	sameSite := http.SameSiteLaxMode
	if secure {
		// Cross-site web (thebillgenie.com → fly.dev) needs None+Secure.
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     "/auth",
		MaxAge:   maxAgeSeconds,
		Expires:  time.Now().Add(time.Duration(maxAgeSeconds) * time.Second),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func clearRefreshTokenCookie(c *gin.Context) {
	secure := refreshCookieSecure()
	sameSite := http.SameSiteLaxMode
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     "/auth",
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})
}

func readRefreshToken(c *gin.Context, bodyToken string) string {
	if t := strings.TrimSpace(bodyToken); t != "" {
		return t
	}
	cookie, err := c.Request.Cookie(refreshCookieName)
	if err != nil || cookie == nil {
		return ""
	}
	return strings.TrimSpace(cookie.Value)
}
