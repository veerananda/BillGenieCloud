package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"restaurant-api/internal/config"
	"restaurant-api/internal/handlers"
	"restaurant-api/internal/middleware"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  No .env file found, using environment variables")
	}
}

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Control logging output
	if !cfg.EnableLogging {
		log.SetOutput(io.Discard)
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize database
	db := config.InitializeDatabase(cfg)
	if db == nil {
		log.Fatal("❌ Failed to initialize database")
	}

	// Run migrations
	config.MigrateDatabase(db)

	// Create router
	router := gin.New()
	router.Use(gin.Recovery())
	if cfg.EnableLogging {
		router.Use(gin.Logger())
	}

	// Setup gzip compression middleware (must be early)
	router.Use(middleware.GzipMiddleware())

	// Setup CORS middleware
	router.Use(middleware.CORSMiddleware(cfg.CORSAllowedOrigins))

	// Setup logging middleware
	if cfg.EnableLogging {
		router.Use(middleware.LoggingMiddleware())
	}

	// Initialize WebSocket hub
	wsHub := handlers.NewWebSocketHub()
	handlers.SetGlobalHub(wsHub)
	go wsHub.Run()

	// Setup routes
	handlers.SetupAuthRoutes(router, db)
	handlers.SetupOrderRoutes(router, db)
	handlers.SetupInventoryRoutes(router, db)
	handlers.SetupMenuRoutes(router, db)
	handlers.SetupRestaurantRoutes(router, db)
	handlers.SetupTableRoutes(router, db)
	handlers.SetupUserRoutes(router, db)
	handlers.SetupIngredientRoutes(router, db)
	handlers.SetupPublicRoutes(router, db)

	// WebSocket route with authentication (token via query param for WebSocket compatibility)
	authService := services.NewAuthService(db, "your-secret-key")
	router.GET("/ws", func(c *gin.Context) {
		// Get token from query parameter
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		// Validate token
		claims, err := authService.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// Set context values for WebSocket handler
		c.Set("user_id", claims.UserID)
		c.Set("restaurant_id", claims.RestaurantID)
		c.Set("role", claims.Role)

		handlers.HandleWebSocket(c, wsHub)
	})

	// Health check already registered in auth routes
	// router.GET("/health", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{
	// 		"status": "ok",
	// 		"service": "restaurant-api",
	// 		"version": "1.0.0",
	// 	})
	// })

	// Start server with graceful shutdown
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("✅ Server starting on port %s (Environment: %s)", cfg.ServerPort, cfg.Environment)
	log.Printf("📡 WebSocket available at ws://localhost:%s/ws", cfg.ServerPort)
	log.Printf("🏥 Health check at http://localhost:%s/health", cfg.ServerPort)

	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Channel to listen for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-quit
	log.Println("🛑 Shutting down server gracefully...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("❌ Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server stopped")
}
