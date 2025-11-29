package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	// Database
	DatabaseURL string
	DBHost      string
	DBPort      string
	DBUser      string
	DBPassword  string
	DBName      string
	DBDriver    string

	// Server
	ServerPort      string
	Environment     string
	APIBaseURL      string
	LogLevel        string

	// JWT
	JWTSecret          string
	JWTExpiry          time.Duration
	RefreshTokenExpiry time.Duration
	RefreshJWTSecret   string

	// WebSocket
	WebSocketPort      string
	WebSocketReadBuf   int
	WebSocketWriteBuf  int

	// Razorpay
	RazorpayKeyID     string
	RazorpayKeySecret string

	// CORS
	CORSAllowedOrigins []string

	// Features
	EnablePayment   bool
	EnableWebSocket bool
	EnableLogging   bool
}

func LoadConfig() *Config {
	cfg := &Config{
		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://user:password@localhost:5432/restaurant_db"),
		DBHost:      getEnv("DATABASE_HOST", "localhost"),
		DBPort:      getEnv("DATABASE_PORT", "5432"),
		DBUser:      getEnv("DATABASE_USER", "user"),
		DBPassword:  getEnv("DATABASE_PASSWORD", "password"),
		DBName:      getEnv("DATABASE_NAME", "restaurant_db"),
		DBDriver:    getEnv("DATABASE_DRIVER", "postgres"),

		// Server
		ServerPort:  getEnv("PORT", getEnv("SERVER_PORT", "3000")), // Heroku sets PORT, fallback to SERVER_PORT
		Environment: getEnv("SERVER_ENV", "development"),
		APIBaseURL:  getEnv("API_BASE_URL", "http://localhost:3000"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),

		// JWT
		JWTSecret:          getEnv("JWT_SECRET", "your-secret-key-change-this"),
		JWTExpiry:          parseDuration(getEnv("JWT_EXPIRY", "24h")),
		RefreshTokenExpiry: parseDuration(getEnv("REFRESH_TOKEN_EXPIRY", "7d")),
		RefreshJWTSecret:   getEnv("REFRESH_JWT_SECRET", "your-refresh-secret-key"),

		// WebSocket
		WebSocketPort:    getEnv("WEBSOCKET_PORT", "3001"),
		WebSocketReadBuf:  getIntEnv("WEBSOCKET_READ_BUFFER", 1024),
		WebSocketWriteBuf: getIntEnv("WEBSOCKET_WRITE_BUFFER", 1024),

		// Razorpay
		RazorpayKeyID:     getEnv("RAZORPAY_KEY_ID", ""),
		RazorpayKeySecret: getEnv("RAZORPAY_KEY_SECRET", ""),

		// CORS
		CORSAllowedOrigins: parseOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,exp://localhost:19000")),

		// Features
		EnablePayment:   getBoolEnv("ENABLE_PAYMENT", true),
		EnableWebSocket: getBoolEnv("ENABLE_WEBSOCKET", true),
		EnableLogging:   getBoolEnv("ENABLE_LOGGING", true),
	}

	log.Printf("✅ Configuration loaded (Environment: %s)", cfg.Environment)
	return cfg
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		// Try to parse as int
		result := 0
		for i := 0; i < len(value); i++ {
			if value[i] < '0' || value[i] > '9' {
				return defaultValue
			}
			result = result*10 + int(value[i]-'0')
		}
		return result
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return strings.ToLower(value) == "true" || value == "1"
}

func parseDuration(value string) time.Duration {
	duration, err := time.ParseDuration(value)
	if err != nil {
		log.Printf("⚠️  Failed to parse duration %s, using default", value)
		return 24 * time.Hour
	}
	return duration
}

func parseOrigins(origins string) []string {
	if origins == "" {
		return []string{"*"}
	}
	return strings.Split(origins, ",")
}
