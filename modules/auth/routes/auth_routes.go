package routes

import (
	"josex/web/modules/auth/controllers"
	"josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"

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
		authRoutes.POST("/token/refresh", authController.RefreshToken)

		// Email verification (public - requires token in body)
		authRoutes.PUT("/email/verification", authController.ConfirmEmailAddress)

		// Password reset (public - requires token in body)
		authRoutes.POST("/password/reset", authController.GeneratePasswordResetToken)
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
