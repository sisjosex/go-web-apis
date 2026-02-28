package routes

import (
	"josex/web/modules/auth/config"
	otpControllers "josex/web/modules/auth/controllers"
	otpRepos "josex/web/modules/auth/repositories"
	otpServices "josex/web/modules/auth/services"
	otpProviders "josex/web/modules/auth/services/otp"
	coreServices "josex/web/modules/core/services"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

// SetupOtpRoutes configures all OTP-related routes
func SetupOtpRoutes(
	authGroup *gin.RouterGroup,
	dbService coreServices.DatabaseService,
	authConfig *config.AuthConfig,
	jwtService otpServices.JWTService,
) {
	// Initialize OTP providers (WhatsApp, SMS, Email)
	whatsappProvider := otpProviders.NewWhatsAppProvider(authConfig)
	smsProvider := otpProviders.NewSmsProvider(authConfig)
	emailProvider := otpProviders.NewEmailProvider(authConfig)

	// Create provider registry
	providers := map[string]otpProviders.OtpProvider{
		"whatsapp": whatsappProvider,
		"sms":      smsProvider,
		"email":    emailProvider,
	}

	// Initialize repository
	otpRepository := otpRepos.NewOtpRepository(dbService)

	// Initialize service
	otpService := otpServices.NewOtpService(otpRepository, providers, authConfig)

	// Initialize parser for User-Agent
	parser, err := uaparser.New("./config/regexes.yaml")
	if err != nil {
		log.Printf("⚠️  Warning: Could not load UA parser regexes: %v (using nil parser)", err)
		parser = nil
	}

	// Initialize controller
	otpController := otpControllers.NewOtpController(otpService, jwtService, parser)

	// Setup routes
	// OTP Request endpoints (per channel)
	authGroup.POST("/otp/whatsapp/request", otpController.RequestOtpWhatsApp)
	authGroup.POST("/otp/sms/request", otpController.RequestOtpSms)
	authGroup.POST("/otp/email/request", otpController.RequestOtpEmail)

	// OTP Verification (unified endpoint for all channels)
	authGroup.POST("/otp/verify", otpController.VerifyOtp)
}
