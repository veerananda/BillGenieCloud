package middleware

import (
	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

// GzipMiddleware enables gzip compression for responses.
// SSE streams are excluded — compressing them breaks EventSource / print-agent clients.
func GzipMiddleware() gin.HandlerFunc {
	return gzip.Gzip(
		gzip.DefaultCompression,
		gzip.WithExcludedPathsRegexs([]string{
			`/events$`,
			`^/print-agent/events`,
		}),
	)
}
