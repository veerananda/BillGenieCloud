package handlers

// appJWTSecret is set once at startup from config (main.go).
var appJWTSecret string

// SetJWTSecret configures the shared JWT secret for all route handlers.
func SetJWTSecret(secret string) {
	appJWTSecret = secret
}
