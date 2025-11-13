package main

import (
	"fmt"
	"log"

	"github.com/joho/godotenv"
	"github.com/gin-gonic/gin"
	"restaurant-api/internal/config"
	"restaurant-api/internal/handlers"
	"restaurant-api/internal/middleware"
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
	router := gin.Default()

	// Setup CORS middleware
	router.Use(middleware.CORSMiddleware(cfg.CORSAllowedOrigins))

	// Setup logging middleware
	router.Use(middleware.LoggingMiddleware())

	// Initialize WebSocket hub
	wsHub := handlers.NewWebSocketHub()
	go wsHub.Run()

	// Setup routes
	handlers.SetupAuthRoutes(router, db)
	handlers.SetupOrderRoutes(router, db)
	handlers.SetupInventoryRoutes(router, db)
	handlers.SetupMenuRoutes(router, db)
	handlers.SetupRestaurantRoutes(router, db)
	handlers.SetupUserRoutes(router, db)
	handlers.SetupPublicRoutes(router, db)

	// WebSocket route
	router.GET("/ws", func(c *gin.Context) {
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

	// 404 handler
	router.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error": "Route not found",
			"path": c.Request.URL.Path,
		})
	})

	// Start server
	addr := fmt.Sprintf(":%s", cfg.ServerPort)
	log.Printf("✅ Server starting on http://localhost:%s", cfg.ServerPort)
	log.Printf("📡 WebSocket available at ws://localhost:%s/ws", cfg.ServerPort)
	log.Printf("🏥 Health check at http://localhost:%s/health", cfg.ServerPort)

	if err := router.Run(addr); err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}
