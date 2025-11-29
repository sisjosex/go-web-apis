package config

import (
	"josex/web/modules/core/utils"
	"time"
)

// AuthConfig holds configuration for auth module
type AuthConfig struct {
	// JWT
	JWTSecretKey         string
	JWTRefreshKey        string
	JWTExpiration        time.Duration
	JWTRefreshExpiration time.Duration

	// Email verification
	EmailVerificationTokenExpiry time.Duration
	EmailVerificationRequired    bool

	// Password reset
	PasswordResetTokenExpiry time.Duration

	// Session
	MaxSessionsPerUser int

	// OAuth providers
	EnableFacebookAuth bool
	EnableGoogleAuth   bool

	// SMTP configuration
	SMTPHost string
	SMTPPort int
	SMTPUser string
	SMTPPass string
	SMTPFrom string
}

// LoadAuthConfig loads auth configuration from environment
func LoadAuthConfig() *AuthConfig {
	return &AuthConfig{
		// JWT configuration
		JWTSecretKey:         utils.MustGetEnv("JWT_SECRET_KEY"),  // Required
		JWTRefreshKey:        utils.MustGetEnv("JWT_REFRESH_KEY"), // Required
		JWTExpiration:        utils.GetEnvAsDuration("JWT_EXPIRATION_MINUTES", 15*time.Minute),
		JWTRefreshExpiration: utils.GetEnvAsDuration("JWT_REFRESH_EXPIRATION_HOURS", 168*time.Hour), // 7 days

		// Email verification
		EmailVerificationTokenExpiry: utils.GetEnvAsDuration("EMAIL_VERIFICATION_TOKEN_EXPIRY_HOURS", 24*time.Hour),
		EmailVerificationRequired:    utils.GetEnvAsBool("EMAIL_VERIFICATION_REQUIRED", false),

		// Password reset
		PasswordResetTokenExpiry: utils.GetEnvAsDuration("PASSWORD_RESET_TOKEN_EXPIRY_HOURS", 1*time.Hour),

		// Session management
		MaxSessionsPerUser: utils.GetEnvAsInt("MAX_SESSIONS_PER_USER", 5),

		// OAuth providers
		EnableFacebookAuth: utils.GetEnvAsBool("ENABLE_FACEBOOK_AUTH", false),
		EnableGoogleAuth:   utils.GetEnvAsBool("ENABLE_GOOGLE_AUTH", false),

		// SMTP configuration for emails
		SMTPHost: utils.GetEnv("SMTP_HOST", "mail.smtp2go.com"),
		SMTPPort: utils.GetEnvAsInt("SMTP_PORT", 2525),
		SMTPUser: utils.GetEnv("SMTP_USER", ""),
		SMTPPass: utils.GetEnv("SMTP_PASS", ""),
		SMTPFrom: utils.GetEnv("SMTP_FROM", "noreply@example.com"),
	}
}
