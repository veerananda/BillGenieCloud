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
		public.POST("/auth/refresh", authHandler.RefreshToken)
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
		protected.PUT("/:order_id", orderHandler.UpdateOrder)
		protected.PUT("/:order_id/complete", orderHandler.CompleteOrder)
		protected.POST("/:order_id/complete-payment", orderHandler.CompleteOrderWithPayment)
		protected.PUT("/:order_id/cancel", orderHandler.CancelOrder)
		protected.PUT("/:order_id/items/:item_id/status", orderHandler.UpdateOrderItemStatus)
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

	// All menu routes require authentication since they need restaurant_id
	protected := router.Group("/menu")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		// Read operations - all authenticated users
		protected.GET("", menuHandler.GetMenuItems)
		protected.GET("/:menu_item_id", menuHandler.GetMenuItem)

		// Write operations - admin/manager only
		protected.POST("", middleware.RoleMiddleware("admin", "manager"), menuHandler.CreateMenuItem)
		protected.PUT("/:menu_item_id", middleware.RoleMiddleware("admin", "manager"), menuHandler.UpdateMenuItem)
		protected.DELETE("/:menu_item_id", middleware.RoleMiddleware("admin", "manager"), menuHandler.DeleteMenuItem)
		protected.PUT("/:menu_item_id/toggle", middleware.RoleMiddleware("admin", "manager"), menuHandler.ToggleAvailability)
	}

	log.Println("✅ Menu routes registered")
}

// SetupRestaurantRoutes registers restaurant endpoints
func SetupRestaurantRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	restaurantHandler := NewRestaurantHandler(db)

	protected := router.Group("/restaurants")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/profile", restaurantHandler.GetRestaurantProfile)
		protected.PUT("/profile", restaurantHandler.UpdateRestaurantProfile)
	}

	log.Println("✅ Restaurant routes registered")
}

// SetupTableRoutes registers table management endpoints
func SetupTableRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	tableHandler := NewTableHandler(db)

	// IMPORTANT: Register more specific routes BEFORE generic routes
	// /tables/bulk must come before /tables POST to avoid route conflicts

	// Create multiple tables - Register with explicit path
	tablesBulk := router.Group("/tables")
	tablesBulk.Use(middleware.AuthMiddleware(authService))
	tablesBulk.Use(middleware.RoleMiddleware("admin", "manager"))
	{
		tablesBulk.POST("/bulk", tableHandler.CreateBulkTables)
	}

	// All other /tables routes
	protected := router.Group("/tables")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		// Get all tables (auth only)
		protected.GET("", tableHandler.GetTables)
	}

	// Occupy/Vacant operations - auth only (any staff can manage table status)
	tableStatus := router.Group("/tables")
	tableStatus.Use(middleware.AuthMiddleware(authService))
	{
		tableStatus.PUT("/:id/occupy", tableHandler.SetTableOccupied)
		tableStatus.PUT("/:id/vacant", tableHandler.SetTableVacant)
	}

	// Create single table and modify operations
	tablesOps := router.Group("/tables")
	tablesOps.Use(middleware.AuthMiddleware(authService))
	tablesOps.Use(middleware.RoleMiddleware("admin", "manager"))
	{
		tablesOps.POST("", tableHandler.CreateTable)
		tablesOps.PUT("/:id", tableHandler.UpdateTable)
		tablesOps.DELETE("/:id", tableHandler.DeleteTable)
	}

	log.Println("✅ Table routes registered")
	log.Println("   📍 POST   /tables/bulk (admin/manager required)")
	log.Println("   📍 GET    /tables (auth required)")
	log.Println("   📍 POST   /tables (admin/manager required)")
	log.Println("   📍 PUT    /tables/:id (admin/manager required)")
	log.Println("   📍 DELETE /tables/:id (admin/manager required)")
	log.Println("   📍 PUT    /tables/:id/occupy (auth required - any role)")
	log.Println("   📍 PUT    /tables/:id/vacant (auth required - any role)")
}

// SetupUserRoutes registers user/staff endpoints
func SetupUserRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	userService := services.NewUserService(db)
	userHandler := NewUserHandler(userService)

	protected := router.Group("/users")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(middleware.RoleMiddleware("admin"))
	{
		// List all staff users for the restaurant
		protected.GET("", userHandler.ListUsers)

		// Get statistics about staff
		protected.GET("/stats", userHandler.GetStaffStats)

		// Create a new staff or manager user
		protected.POST("", userHandler.CreateUser)

		// Get specific user details
		protected.GET("/:user_id", userHandler.GetUser)

		// Update a staff user
		protected.PUT("/:user_id", userHandler.UpdateUser)

		// Delete (soft-delete) a staff user
		protected.DELETE("/:user_id", userHandler.DeleteUser)

		// Restore a deleted staff user
		protected.POST("/:user_id/restore", userHandler.RestoreUser)
	}

	log.Println("✅ User routes registered with full implementation")
}

// SetupIngredientRoutes registers ingredient endpoints
func SetupIngredientRoutes(router *gin.Engine, db *gorm.DB) {
	authService := services.NewAuthService(db, "your-secret-key") // TODO: Load from .env
	ingredientHandler := NewIngredientHandler(db)

	protected := router.Group("/ingredients")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("", ingredientHandler.ListIngredients)
		protected.POST("", ingredientHandler.CreateIngredient)
		protected.PUT("/:ingredient_id", ingredientHandler.UpdateIngredient)
		protected.DELETE("/:ingredient_id", ingredientHandler.DeleteIngredient)
	}

	log.Println("✅ Ingredient routes registered")
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
