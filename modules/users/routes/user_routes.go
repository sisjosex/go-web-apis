package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	coreMiddleware "josex/web/modules/core/middleware"
	"josex/web/modules/users/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes registra todas las rutas del módulo de usuarios (admin)
func RegisterUserRoutes(
	router *gin.RouterGroup,
	userController *controllers.UserController,
	jwtService services.JWTService,
) {
	// Todas las rutas de usuarios requieren autenticación Y rol de admin o super_admin
	userRoutes := router.Group("/users")
	userRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	userRoutes.Use(coreMiddleware.RequireSystemRole("admin", "super_admin")) // 🔒 PROTECTED
	{
		// CRUD operations (admin/super_admin only)
		userRoutes.GET("", userController.ListUsers)             // List all users
		userRoutes.POST("", userController.Create)               // Create new user
		userRoutes.GET("/:id", userController.GetUserById)       // Get user by ID
		userRoutes.PUT("/:id", userController.Update)            // Update user
		userRoutes.DELETE("/:id", userController.SoftDeleteUser) // Soft delete user
	}
}
