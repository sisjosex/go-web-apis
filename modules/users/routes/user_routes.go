package routes

import (
	authMiddleware "josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/users/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterUserRoutes registra todas las rutas del módulo de usuarios (admin)
func RegisterUserRoutes(
	router *gin.RouterGroup,
	userController *controllers.UserController,
	jwtService services.JWTService,
) {
	// Todas las rutas de usuarios requieren autenticación
	userRoutes := router.Group("/users")
	userRoutes.Use(authMiddleware.AuthMiddleware(jwtService))
	{
		// CRUD operations
		userRoutes.GET("", userController.ListUsers)             // List all users
		userRoutes.POST("", userController.Create)               // Create new user
		userRoutes.GET("/:id", userController.GetUserById)       // Get user by ID
		userRoutes.PUT("/:id", userController.Update)            // Update user
		userRoutes.DELETE("/:id", userController.SoftDeleteUser) // Soft delete user
	}
}
