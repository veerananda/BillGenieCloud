package handlers

// appJWTSecret and appRefreshJWTSecret are set once at startup from config (main.go).
var appJWTSecret string
var appRefreshJWTSecret string

// SetJWTSecrets configures shared JWT secrets for all route handlers.
func SetJWTSecrets(accessSecret, refreshSecret string) {
	appJWTSecret = accessSecret
	appRefreshJWTSecret = refreshSecret
}

// SetJWTSecret configures the access JWT secret (kept for compatibility).
func SetJWTSecret(secret string) {
	appJWTSecret = secret
}
