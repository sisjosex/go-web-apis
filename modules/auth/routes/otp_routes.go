package routes

import (
	"josex/web/modules/auth/config"
	otpControllers "josex/web/modules/auth/controllers"
	otpRepos "josex/web/modules/auth/repositories"
	otpServices "josex/web/modules/auth/services"
	otpProviders "josex/web/modules/auth/services/otp"
	"josex/web/modules/core/api"
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
	hooks ...otpServices.SessionHook,
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
	otpService := otpServices.NewOtpService(otpRepository, providers, authConfig, hooks...)

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
	api.Post(authGroup, "/otp/whatsapp/request", otpController.RequestOtpWhatsApp, api.Doc{Summary: "Request OTP via WhatsApp (relayed by email while it has no provider)", Errors: []int{400, 500, 503}})
	api.Post(authGroup, "/otp/sms/request", otpController.RequestOtpSms, api.Doc{Summary: "Request OTP via SMS (relayed by email while it has no provider)", Errors: []int{400, 500, 503}})
	api.Post(authGroup, "/otp/email/request", otpController.RequestOtpEmail, api.Doc{Summary: "Request OTP via Email", Errors: []int{400, 500, 503}})

	// OTP Verification (unified endpoint for all channels)
	api.Post(authGroup, "/otp/verify", otpController.VerifyOtp, api.Doc{Summary: "Verify OTP code", Errors: []int{400, 500}})
}
