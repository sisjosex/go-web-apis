package routes

import (
	"josex/web/modules/auth/config"
	"josex/web/modules/auth/controllers"
	"josex/web/modules/auth/middleware"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
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
		api.Post(authRoutes, "/login", authController.Login, api.Doc{Summary: "Allow login based on email authentication, email can be confirmed after login", Description: "Allow login based on email authentication, email can be confirmed after login", Errors: []int{400}})
		api.Post(authRoutes, "/login/facebook", authController.LoginFacebook, api.Doc{Summary: "Allow login based on Facebook authentication", Description: "Allow login based on Facebook authentication", Errors: []int{400}})
		api.Post(authRoutes, "/login/google", authController.LoginGoogle, api.Doc{Summary: "Sign in with Google", Description: "Exchanges the code from the Google popup (GIS code flow, ux_mode popup) or, with X-Client-Type mobile, the Android server auth code (MOBILE-035), checks the ID token names this app and a verified address, then signs in the account with that email or creates it (APP-016 D4). 503 when the server has no Google client.", Errors: []int{400, 401, 503}})
		api.Post(authRoutes, "/register", authController.Register, api.Doc{Summary: "Register", Description: "Register", Errors: []int{400}})
		api.Post(authRoutes, "/refresh_token", authController.RefreshToken, api.Doc{Summary: "Refresh access token", Description: "Refresh access token using refresh token and validate active session", Errors: []int{400, 401}})

		// Email verification (public - requires token in body)
		api.Put(authRoutes, "/email/verification", authController.ConfirmEmailAddress, api.Doc{Summary: "Confirm email address", Description: "Confirm email address with verification token", Errors: []int{400}})

		// Password reset (public - requires token in body/query)
		api.Post(authRoutes, "/password/reset", authController.GeneratePasswordResetToken, api.Doc{Summary: "Request password reset", Description: "Generate and send password reset token The answer is `true` whether or not the email has an account, and never carries the token: the link in the email is the only way to it.", Errors: []int{400}})
		api.Get(authRoutes, "/password/reset", authController.ValidateResetToken, api.Doc{Summary: "Validate password reset token", Description: "Check whether a password reset token is still valid (not used, not expired)", Errors: []int{400, 410}})
		api.Put(authRoutes, "/password/reset", authController.ResetPasswordWithToken, api.Doc{Summary: "Reset password with token", Description: "Reset password using reset token", Errors: []int{400}})

		// ========================================
		// Protected Routes (require authentication)
		// ========================================
		// Use adds to authRoutes itself: every route registered on it from here carries it.
		authRoutes.Use(middleware.AuthMiddleware(jwtService))
		protectedRoutes := authRoutes
		{
			// Profile management
			api.Get(protectedRoutes, "/profile", authController.GetProfile, api.Doc{Summary: "Get user profile", Description: "Get authenticated user profile information", Errors: []int{400}})
			api.Patch(protectedRoutes, "/profile", authController.UpdateProfile, api.Doc{Summary: "Update user profile", Description: "Update authenticated user profile information (partial updates allowed)", Errors: []int{400}})

			// Password management
			api.Put(protectedRoutes, "/password", authController.ChangePassword, api.Doc{Summary: "Change user password", Description: "Change password for authenticated user", Errors: []int{400}})

			// Email verification request (authenticated user)
			api.Post(protectedRoutes, "/email/verification", authController.GenerateEmailVerificationToken, api.Doc{Summary: "Request email verification", Description: "Generate and send email verification token", Errors: []int{400, 503}})

			// Session management
			api.Get(protectedRoutes, "/sessions", sessionController.GetActiveSessions, api.Doc{Summary: "Get active sessions", Description: "Get all sessions (active and inactive) for authenticated user", Errors: []int{400, 401}})
			api.Delete(protectedRoutes, "/sessions/{id}", sessionController.LogoutSession, api.Doc{Summary: "Logout specific session", Description: "Logout a specific session by ID (must belong to authenticated user)", Errors: []int{400, 401, 404}})
			api.Delete(protectedRoutes, "/sessions", sessionController.LogoutAllSessions, api.Doc{Summary: "Logout all sessions", Description: "Logout all sessions for authenticated user except current one", Errors: []int{400, 401}})
			api.Post(protectedRoutes, "/logout", authController.Logout, api.Doc{Summary: "Logout", Description: "Logout", Errors: []int{400}})
		}
	}
}

// RegisterPushDeviceRoutes mounts /mobile/devices (TRACK-012): auth only, no tenant — a phone is the
// account's, whatever tenants the account belongs to.
func RegisterPushDeviceRoutes(apiV1Group *gin.RouterGroup, controller *controllers.PushDeviceController, jwtService services.JWTService) {
	devices := apiV1Group.Group("/mobile/devices", middleware.AuthMiddleware(jwtService))
	api.Post(devices, "", controller.RegisterDevice, api.Doc{Summary: "Register or refresh this phone's push token", Description: "Upsert on (account, device_id): send it on every app start, the token rotates. A token registered under another account moves to this one", Status: 204, Errors: []int{400, 401, 500}})
	api.Delete(devices, "/{device_id}", controller.ForgetDevice, api.Doc{Summary: "Stop pushing to this phone", Description: "Call on logout. An unknown device_id is not an error", Status: 204, Errors: []int{401, 500}})
}

// RegisterOtpRoutes registers all OTP-related routes
// This is called from the main routes setup with auth config and database service
func RegisterOtpRoutes(
	apiV1Group *gin.RouterGroup,
	dbService coreServices.DatabaseService,
	authConfig *config.AuthConfig,
	jwtService services.JWTService,
	hooks ...services.SessionHook,
) {
	// Create auth routes group for OTP routes
	authGroup := apiV1Group.Group("/auth")
	{
		// Setup OTP routes using the function from otp_routes.go
		SetupOtpRoutes(authGroup, dbService, authConfig, jwtService, hooks...)
	}
}
