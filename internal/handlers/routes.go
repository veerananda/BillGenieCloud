package handlers

import (
	"log"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupAuthRoutes registers authentication endpoints
func SetupAuthRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	authHandler := NewAuthHandler(authService)

	public := router.Group("")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.GET("/health", authHandler.HealthCheck)
	}

	protected := router.Group("")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/auth/profile", authHandler.GetProfile)
	}

	log.Println("✅ Auth routes registered")
}

// SetupOrderRoutes registers order endpoints
func SetupOrderRoutes(router *gin.Engine, db *gorm.DB) {
	orderService := services.NewOrderService(db)
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	orderHandler := NewOrderHandler(orderService)

	protected := router.Group("/orders")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.POST("", orderHandler.CreateOrder)
		protected.GET("", orderHandler.ListOrders)
		protected.GET("/:order_id", orderHandler.GetOrder)
		protected.PUT("/:order_id/complete", orderHandler.CompleteOrder)
		protected.PUT("/:order_id/cancel", orderHandler.CancelOrder)
	}

	log.Println("✅ Order routes registered")
}

// SetupInventoryRoutes registers inventory endpoints
func SetupInventoryRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	inventoryHandler := NewInventoryHandler(db)

	protected := router.Group("/inventory")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("", inventoryHandler.GetInventory)
		protected.GET("/alerts", inventoryHandler.GetLowStockAlert)
		protected.PUT("/:menu_item_id", inventoryHandler.UpdateInventory)
		protected.POST("/deduct", inventoryHandler.DeductInventory)
		protected.POST("/restock", inventoryHandler.RestockInventory)
	}

	log.Println("✅ Inventory routes registered")
}

// SetupMenuRoutes registers menu endpoints
func SetupMenuRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	menuHandler := NewMenuHandler(db)

	public := router.Group("/menu")
	{
		public.GET("", menuHandler.GetMenuItems)
		public.GET("/:menu_item_id", menuHandler.GetMenuItem)
	}

	protected := router.Group("/menu")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(middleware.RoleMiddleware("admin", "manager"))
	{
		protected.POST("", menuHandler.CreateMenuItem)
		protected.PUT("/:menu_item_id", menuHandler.UpdateMenuItem)
		protected.DELETE("/:menu_item_id", menuHandler.DeleteMenuItem)
		protected.PUT("/:menu_item_id/toggle", menuHandler.ToggleAvailability)
	}

	log.Println("✅ Menu routes registered")
}

// SetupRestaurantRoutes registers restaurant endpoints
func SetupRestaurantRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env

	protected := router.Group("/restaurants")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/profile", func(c *gin.Context) {
			// Get restaurant profile
			restaurantID, _ := c.Get("restaurant_id")
			c.JSON(200, gin.H{
				"message":       "Restaurant profile",
				"restaurant_id": restaurantID,
			})
		})

		protected.PUT("/profile", func(c *gin.Context) {
			// Update restaurant profile
			c.JSON(200, gin.H{
				"message": "Restaurant profile updated",
			})
		})
	}

	log.Println("✅ Restaurant routes registered")
}

// SetupUserRoutes registers user/staff endpoints
func SetupUserRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env

	protected := router.Group("/users")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(middleware.RoleMiddleware("admin"))
	{
		protected.GET("", func(c *gin.Context) {
			// List staff users
			restaurantID, _ := c.Get("restaurant_id")
			c.JSON(200, gin.H{
				"message":       "Staff list",
				"restaurant_id": restaurantID,
			})
		})

		protected.POST("", func(c *gin.Context) {
			// Create new staff user
			c.JSON(201, gin.H{
				"message": "Staff user created",
			})
		})

		protected.PUT("/:user_id", func(c *gin.Context) {
			// Update staff user
			userID := c.Param("user_id")
			c.JSON(200, gin.H{
				"message": "Staff user updated",
				"user_id": userID,
			})
		})

		protected.DELETE("/:user_id", func(c *gin.Context) {
			// Delete staff user
			userID := c.Param("user_id")
			c.JSON(200, gin.H{
				"message": "Staff user deleted",
				"user_id": userID,
			})
		})
	}

	log.Println("✅ User routes registered")
}

// SetupPublicRoutes registers public endpoints (no authentication required)
func SetupPublicRoutes(router *gin.Engine, db *gorm.DB) {
	publicHandler := NewPublicHandler(db)
	
	public := router.Group("/public")
	{
		// Menu endpoints - accessible without authentication
		public.GET("/menu", publicHandler.GetPublicMenu)
		public.GET("/menu/:menu_item_id", publicHandler.GetPublicMenuItem)
		
		// Restaurant info endpoint
		public.GET("/restaurant", publicHandler.GetPublicRestaurant)
	}
	
	log.Println("✅ Public routes registered")
}
