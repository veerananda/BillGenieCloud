package handlers

import (
	"log"

	"restaurant-api/internal/middleware"
	"restaurant-api/internal/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func getAuthService(db *gorm.DB) *services.AuthService {
	secret := appJWTSecret
	if secret == "" {
		secret = "your-secret-key-change-this"
	}
	return services.NewAuthService(db, secret)
}

func withSubscription(db *gorm.DB) gin.HandlerFunc {
	return middleware.SubscriptionMiddleware(db)
}

// SetupAuthRoutes registers authentication endpoints
func SetupAuthRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	authHandler := NewAuthHandler(authService)

	public := router.Group("")
	{
		public.POST("/auth/register", authHandler.Register)
		public.POST("/auth/login", authHandler.Login)
		public.POST("/auth/refresh", authHandler.RefreshToken)
		public.POST("/auth/forgot-password", authHandler.ForgotPassword)
		public.GET("/reset-password", authHandler.ResetPasswordPage)
		public.POST("/auth/reset-password", authHandler.ResetPassword)
		public.POST("/auth/forgot-login-id", authHandler.ForgotLoginID)
		public.POST("/auth/verify-login-recovery", authHandler.VerifyLoginRecovery)
		public.POST("/auth/verify-email", authHandler.VerifyEmail)
		public.GET("/auth/verification-status", authHandler.GetVerificationStatus)
		public.POST("/auth/resend-verification", authHandler.ResendVerificationEmail)
		public.GET("/health", authHandler.HealthCheck)
	}

	protected := router.Group("")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/auth/profile", authHandler.GetProfile)
		protected.POST("/auth/logout", authHandler.Logout)
	}

	log.Println("✅ Auth routes registered")
}

// SetupOrderRoutes registers order endpoints
func SetupOrderRoutes(router *gin.Engine, db *gorm.DB) {
	orderService := services.NewOrderService(db)
	checkoutLock := services.NewCheckoutLockService(db)
	authService := getAuthService(db)
	orderHandler := NewOrderHandler(orderService, checkoutLock, authService)

	protected := router.Group("/orders")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	{
		protected.POST("", orderHandler.CreateOrder)
		protected.GET("/summary", orderHandler.ListOrdersSummary)
		protected.GET("/sales-summary", orderHandler.GetSalesSummary)
		protected.GET("/history", orderHandler.ListOrderHistory)
		protected.GET("/counter/next-ticket", orderHandler.GetNextCounterTicket)
		protected.GET("/counter/today", orderHandler.ListCounterOrdersToday)
		protected.GET("", orderHandler.ListOrders)
		protected.GET("/:order_id", orderHandler.GetOrder)
		protected.PUT("/:order_id", orderHandler.UpdateOrder)
		protected.PUT("/:order_id/complete", orderHandler.CompleteOrder)
		protected.POST("/:order_id/complete-payment", orderHandler.CompleteOrderWithPayment)
		protected.POST("/:order_id/checkout/start", orderHandler.StartCheckout)
		protected.POST("/:order_id/checkout/cancel", orderHandler.CancelCheckout)
		protected.PUT("/:order_id/cancel", orderHandler.CancelOrder)
		protected.PUT("/:order_id/items/:item_id/status", orderHandler.UpdateOrderItemStatus)
		protected.PUT("/:order_id/menu-items/:menu_id/status", orderHandler.UpdateOrderItemsByMenuID)
		protected.POST("/:order_id/bill-share", orderHandler.CreateBillShare)
	}

	log.Println("✅ Order routes registered")
}

// SetupInventoryRoutes registers inventory endpoints
func SetupInventoryRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	inventoryHandler := NewInventoryHandler(db)

	protected := router.Group("/inventory")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	protected.Use(middleware.RoleMiddleware("admin")) // Admin only — ingredient stock (menu-item level)
	{
		protected.GET("", inventoryHandler.GetInventory)
		protected.GET("/alerts", inventoryHandler.GetLowStockAlert)
		protected.GET("/:menu_item_id", inventoryHandler.GetInventoryByMenuItem)
		protected.PUT("/:menu_item_id", inventoryHandler.UpdateInventory)
		protected.POST("/deduct", inventoryHandler.DeductInventory)
		protected.POST("/restock", inventoryHandler.RestockInventory)
	}

	log.Println("✅ Inventory routes registered (admin/manager only)")
}

// SetupMenuRoutes registers menu endpoints
func SetupMenuRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	menuHandler := NewMenuHandler(db)
	recipeHandler := NewMenuItemIngredientHandler(db)

	// All menu routes require authentication since they need restaurant_id
	protected := router.Group("/menu")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	{
		// Read operations - all authenticated users
		protected.GET("", menuHandler.GetMenuItems)
		protected.GET("/:menu_item_id", menuHandler.GetMenuItem)

		// Write operations - admin/manager only
		protected.POST("", middleware.RoleMiddleware("admin", "manager"), menuHandler.CreateMenuItem)
		protected.PUT("/:menu_item_id", middleware.RoleMiddleware("admin", "manager"), menuHandler.UpdateMenuItem)
		protected.DELETE("/:menu_item_id", middleware.RoleMiddleware("admin", "manager"), menuHandler.DeleteMenuItem)
		protected.PUT("/:menu_item_id/toggle", middleware.RoleMiddleware("admin", "manager"), menuHandler.ToggleAvailability)
		protected.PUT("/:menu_item_id/ingredients", middleware.RoleMiddleware("admin"), recipeHandler.SetMenuItemIngredients)
	}

	log.Println("✅ Menu routes registered")
}

// SetupMenuItemIngredientRoutes registers recipe (BOM) read endpoints
func SetupMenuItemIngredientRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	recipeHandler := NewMenuItemIngredientHandler(db)

	protected := router.Group("/menu-item-ingredients")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	protected.Use(middleware.RoleMiddleware("admin"))
	{
		protected.GET("", recipeHandler.ListMenuItemIngredients)
	}

	log.Println("✅ Menu item ingredient routes registered (admin only)")
}

// SetupRestaurantRoutes registers restaurant endpoints
func SetupRestaurantRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	restaurantHandler := NewRestaurantHandler(db)

	protected := router.Group("/restaurants")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	{
		protected.GET("/profile", restaurantHandler.GetRestaurantProfile)
		protected.PUT("/profile", restaurantHandler.UpdateRestaurantProfile)
	}

	log.Println("✅ Restaurant routes registered")
}

// SetupTableRoutes registers table management endpoints
func SetupTableRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	tableHandler := NewTableHandler(db)

	// IMPORTANT: Register more specific routes BEFORE generic routes
	// /tables/bulk must come before /tables POST to avoid route conflicts

	// Create multiple tables - Register with explicit path
	tablesBulk := router.Group("/tables")
	tablesBulk.Use(middleware.AuthMiddleware(authService))
	tablesBulk.Use(withSubscription(db))
	tablesBulk.Use(middleware.RoleMiddleware("admin", "manager"))
	{
		tablesBulk.POST("/bulk", tableHandler.CreateBulkTables)
	}

	// All other /tables routes
	protected := router.Group("/tables")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	{
		// Get all tables (auth only)
		protected.GET("", tableHandler.GetTables)
	}

	// Occupy/Vacant operations - auth only (any staff can manage table status)
	tableStatus := router.Group("/tables")
	tableStatus.Use(middleware.AuthMiddleware(authService))
	tableStatus.Use(withSubscription(db))
	{
		tableStatus.PUT("/:id/occupy", tableHandler.SetTableOccupied)
		tableStatus.PUT("/:id/vacant", tableHandler.SetTableVacant)
	}

	// Create single table and modify operations
	tablesOps := router.Group("/tables")
	tablesOps.Use(middleware.AuthMiddleware(authService))
	tablesOps.Use(withSubscription(db))
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
	authService := getAuthService(db)
	userService := services.NewUserService(db)
	userHandler := NewUserHandler(userService)

	protected := router.Group("/users")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
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

		// Regenerate staff key for a user
		protected.POST("/:user_id/regenerate-key", userHandler.RegenerateStaffKey)
	}

	log.Println("✅ User routes registered with full implementation")
}

// SetupIngredientRoutes registers ingredient endpoints
func SetupIngredientRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	ingredientHandler := NewIngredientHandler(db)

	protected := router.Group("/ingredients")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(withSubscription(db))
	{
		read := protected.Group("")
		read.Use(middleware.RoleMiddleware("admin", "manager", "chef", "staff"))
		read.GET("", ingredientHandler.ListIngredients)

		restock := protected.Group("")
		restock.Use(middleware.RoleMiddleware("admin", "manager", "chef", "staff"))
		restock.POST("/:ingredient_id/restock", ingredientHandler.RestockIngredient)

		write := protected.Group("")
		write.Use(middleware.RoleMiddleware("admin"))
		write.POST("/sync-from-recipes", ingredientHandler.SyncFromRecipes)
		write.POST("", ingredientHandler.CreateIngredient)
		write.PUT("/:ingredient_id", ingredientHandler.UpdateIngredient)
		write.DELETE("/:ingredient_id", ingredientHandler.DeleteIngredient)
	}

	log.Println("✅ Ingredient routes registered (read: admin/manager/chef, write: admin)")
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

// SetupSubscriptionRoutes registers subscription renewal/payment endpoints.
func SetupSubscriptionRoutes(router *gin.Engine, db *gorm.DB) {
	authService := getAuthService(db)
	subscriptionHandler := NewSubscriptionHandler(db)

	public := router.Group("/subscription")
	{
		public.POST("/signup-quote", subscriptionHandler.QuoteSignupPlan)
	}

	protected := router.Group("/subscription")
	protected.Use(middleware.AuthMiddleware(authService))
	{
		protected.GET("/renewal-quote", subscriptionHandler.GetRenewalQuote)
		protected.POST("/renewal-quote", subscriptionHandler.GetRenewalQuote)
		protected.POST("/create-order", middleware.RoleMiddleware("admin", "manager"), subscriptionHandler.CreateRenewalOrder)
		protected.POST("/verify-payment", middleware.RoleMiddleware("admin", "manager"), subscriptionHandler.VerifyRenewalPayment)
	}

	log.Println("✅ Subscription routes registered")
}

// SetupWebhookRoutes registers public payment provider callbacks.
func SetupWebhookRoutes(router *gin.Engine, db *gorm.DB) {
	webhookHandler := NewSubscriptionWebhookHandler(db)

	router.POST("/webhooks/razorpay", webhookHandler.HandleRazorpayWebhook)

	log.Println("✅ Webhook routes registered")
}

// SetupPlatformRoutes registers BillGenie creator-only operations console API.
func SetupPlatformRoutes(router *gin.Engine, db *gorm.DB) {
	ops := services.NewPlatformOpsService(db)
	platformHandler := NewPlatformHandler(ops)

	platform := router.Group("/platform")
	platform.Use(middleware.PlatformAuthMiddleware())
	{
		platform.GET("/restaurants", platformHandler.ListRestaurants)
		platform.GET("/restaurants/:restaurant_id", platformHandler.GetRestaurant)
		platform.POST("/restaurants/:restaurant_id/grant-subscription", platformHandler.GrantSubscription)
		platform.POST("/restaurants/:restaurant_id/extend-trial", platformHandler.ExtendTrial)
		platform.PUT("/restaurants/:restaurant_id/selection", platformHandler.UpdateSelection)
		platform.PUT("/restaurants/:restaurant_id/active", platformHandler.SetActive)
		platform.DELETE("/restaurants/:restaurant_id", platformHandler.DeleteRestaurant)
		platform.POST("/restaurants/:restaurant_id/menu/bulk", platformHandler.BulkUploadMenu)
		platform.POST("/restaurants/:restaurant_id/recipes/bulk", platformHandler.BulkUploadRecipes)
	}

	log.Println("✅ Platform ops routes registered (/platform/*)")
}
