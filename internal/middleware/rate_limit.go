package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateBucket struct {
	count   int
	resetAt time.Time
}

// RateLimit returns a simple per-process rate limiter keyed by client IP + route.
// Good enough for P0; multi-instance Fly needs Redis-backed limits later.
func RateLimit(maxRequests int, window time.Duration) gin.HandlerFunc {
	var mu sync.Mutex
	buckets := make(map[string]*rateBucket)

	return func(c *gin.Context) {
		key := c.ClientIP() + "|" + c.FullPath()
		now := time.Now()

		mu.Lock()
		b, ok := buckets[key]
		if !ok || now.After(b.resetAt) {
			b = &rateBucket{count: 0, resetAt: now.Add(window)}
			buckets[key] = b
		}
		b.count++
		count := b.count
		resetAt := b.resetAt
		mu.Unlock()

		if count > maxRequests {
			c.Header("Retry-After", resetAt.Sub(now).Round(time.Second).String())
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests, please try again later",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
