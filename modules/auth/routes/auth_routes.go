package routes

import (
	"josex/web/modules/auth/config"
	"josex/web/modules/auth/controllers"
	"josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	coreServices "josex/web/modules/core/services"

	"github.com/gin-gonic/gin"
)

// RegisterAuthRoutes registra todas las rutas del módulo de autenticación
func RegisterAuthRoutes(
	router *gin.RouterGroup,
	authController *controllers.AuthController,
	sessionController *controllers.SessionController,
	jwtService services.JWTService,
) {
	authRoutes := router.Group("/auth")
	{
		// ========================================
		// Public Routes (no authentication)
		// ========================================

		// Login & Registration
		authRoutes.POST("/login", authController.Login)
		authRoutes.POST("/login/facebook", authController.LoginFacebook)
		authRoutes.POST("/register", authController.Register)
		authRoutes.POST("/refresh_token", authController.RefreshToken)

		// Email verification (public - requires token in body)
		authRoutes.PUT("/email/verification", authController.ConfirmEmailAddress)

		// Password reset (public - requires token in body/query)
		authRoutes.POST("/password/reset", authController.GeneratePasswordResetToken)
		authRoutes.GET("/password/reset", authController.ValidateResetToken)
		authRoutes.PUT("/password/reset", authController.ResetPasswordWithToken)

		// ========================================
		// Protected Routes (require authentication)
		// ========================================
		protectedRoutes := authRoutes.Use(middleware.AuthMiddleware(jwtService))
		{
			// Profile management
			protectedRoutes.GET("/profile", authController.GetProfile)
			protectedRoutes.PATCH("/profile", authController.UpdateProfile)

			// Password management
			protectedRoutes.PUT("/password", authController.ChangePassword)

			// Email verification request (authenticated user)
			protectedRoutes.POST("/email/verification", authController.GenerateEmailVerificationToken)

			// Session management
			protectedRoutes.GET("/sessions", sessionController.GetActiveSessions)
			protectedRoutes.DELETE("/sessions/:id", sessionController.LogoutSession)
			protectedRoutes.DELETE("/sessions", sessionController.LogoutAllSessions)
			protectedRoutes.POST("/logout", authController.Logout)
		}
	}
}

// RegisterPushDeviceRoutes mounts /mobile/devices (TRACK-012): auth only, no tenant — a phone is the
// account's, whatever tenants the account belongs to.
func RegisterPushDeviceRoutes(apiV1Group *gin.RouterGroup, controller *controllers.PushDeviceController, jwtService services.JWTService) {
	devices := apiV1Group.Group("/mobile/devices", middleware.AuthMiddleware(jwtService))
	devices.POST("", controller.RegisterDevice)
	devices.DELETE("/:device_id", controller.ForgetDevice)
}

// RegisterOtpRoutes registers all OTP-related routes
// This is called from the main routes setup with auth config and database service
func RegisterOtpRoutes(
	apiV1Group *gin.RouterGroup,
	dbService coreServices.DatabaseService,
	authConfig *config.AuthConfig,
	jwtService services.JWTService,
) {
	// Create auth routes group for OTP routes
	authGroup := apiV1Group.Group("/auth")
	{
		// Setup OTP routes using the function from otp_routes.go
		SetupOtpRoutes(authGroup, dbService, authConfig, jwtService)
	}
}
