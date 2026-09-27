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
	// Provider registry, one entry per channel that can send on its own. SMS and WhatsApp have no paid
	// sender yet (AUTH-001 D2), so the service relays them to the account's email; registering a
	// provider for "sms" or "whatsapp" here ends the relay for that channel.
	emailProvider := otpProviders.NewEmailProvider(authConfig)
	providers := map[string]otpProviders.OtpProvider{
		emailProvider.GetChannelName(): emailProvider,
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
