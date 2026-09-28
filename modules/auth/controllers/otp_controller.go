package controllers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	coreErrors "josex/web/modules/core/errors"
	coreUtils "josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/ua-parser/uap-go/uaparser"
)

// OtpController handles OTP-related requests
type OtpController struct {
	otpService interfaces.OtpService
	jwtService services.JWTService
	parser     *uaparser.Parser
}

// NewOtpController creates a new OTP controller
func NewOtpController(
	otpService interfaces.OtpService,
	jwtService services.JWTService,
	parser *uaparser.Parser,
) *OtpController {
	return &OtpController{
		otpService: otpService,
		jwtService: jwtService,
		parser:     parser,
	}
}

// RequestOtpWhatsApp godoc
// @Summary Request OTP via WhatsApp (relayed by email while it has no provider)
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Invalid destination or channel, or phone without an account"
// @Failure 503 {object} object "Channel disabled or provider unavailable"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/whatsapp/request [post]
func (uc *OtpController) RequestOtpWhatsApp(c *gin.Context) {
	uc.requestOtp(c, "whatsapp")
}

// RequestOtpSms godoc
// @Summary Request OTP via SMS (relayed by email while it has no provider)
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Invalid destination or channel, or phone without an account"
// @Failure 503 {object} object "Channel disabled or provider unavailable"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/sms/request [post]
func (uc *OtpController) RequestOtpSms(c *gin.Context) {
	uc.requestOtp(c, "sms")
}

// RequestOtpEmail godoc
// @Summary Request OTP via Email
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Invalid destination or channel, or phone without an account"
// @Failure 503 {object} object "Channel disabled or provider unavailable"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/email/request [post]
func (uc *OtpController) RequestOtpEmail(c *gin.Context) {
	uc.requestOtp(c, "email")
}

// requestOtp is the body of the three request handlers; the route names the channel.
func (uc *OtpController) requestOtp(c *gin.Context, channel string) {
	var requestDto authModels.RequestOtpRequestDto

	if err := c.ShouldBindJSON(&requestDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpRequestFailed, coreUtils.ExtractValidationError(c, err)))
		return
	}

	userAgent := c.GetHeader("User-Agent")
	var deviceInfo, deviceOs, browser string
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family)
		browser = strings.TrimSpace(client.UserAgent.Family)
	}

	otpDto := authModels.RequestOtpDto{
		Destination: requestDto.Destination,
		Channel:     channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  deviceInfo,
		DeviceOs:    deviceOs,
		Browser:     browser,
		UserAgent:   userAgent,
	}

	// The gin context, not c.Request.Context(): LanguageMiddleware keeps `lang` in gin's keys, and
	// the code's email is written in it.
	response, err := uc.otpService.RequestOtp(c, otpDto)
	if err != nil {
		status, body := requestOtpError(c, err)
		c.JSON(status, body)
		return
	}

	c.JSON(http.StatusOK, response)
}

// requestOtpError maps a request error to its response: the caller's input (an SP code) is 400 with
// that code, a channel that cannot send is 503, anything else 500 without internals.
func requestOtpError(c *gin.Context, err error) (int, *coreErrors.ErrorResponse) {
	if errors.Is(err, services.ErrChannelDisabled) || errors.Is(err, services.ErrProviderUnavailable) {
		return http.StatusServiceUnavailable, coreErrors.BuildErrorSingle(c, err.Error())
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "O") {
		return http.StatusBadRequest, coreErrors.BuildError(c, err)
	}
	log.Printf("OTP request failed: %v", err)
	return http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, authErrors.OtpRequestFailed)
}

// VerifyOtp godoc
// @Summary Verify OTP code
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.VerifyOtpRequestDto true "Verify OTP"
// @Success 200 {object} authModels.VerifyOtpResponse
// @Failure 400 {object} object "Invalid or expired OTP"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/verify [post]
func (uc *OtpController) VerifyOtp(c *gin.Context) {
	var verifyDto authModels.VerifyOtpRequestDto

	if err := c.ShouldBindJSON(&verifyDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpVerifyFailed, coreUtils.ExtractValidationError(c, err)))
		return
	}

	declaredClient, ok := clientType(c)
	if !ok {
		return
	}

	userAgent := c.GetHeader("User-Agent")
	var deviceInfo, deviceOs, browser string
	if uc.parser != nil {
		client := uc.parser.Parse(userAgent)
		deviceInfo = strings.TrimSpace(client.Device.Family)
		deviceOs = strings.TrimSpace(client.Os.Family)
		browser = strings.TrimSpace(client.UserAgent.Family)
	}

	otpVerifyDto := authModels.VerifyOtpDto{
		Destination: verifyDto.Destination,
		OtpCode:     verifyDto.OtpCode,
		Channel:     verifyDto.Channel,
		DeviceId:    verifyDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  deviceInfo,
		DeviceOs:    deviceOs,
		Browser:     browser,
		UserAgent:   userAgent,
		ClientType:  declaredClient,
	}

	response, err := uc.otpService.VerifyOtp(c.Request.Context(), otpVerifyDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}
	// A right code for an email with no active account opens nothing, and says so the way a wrong
	// code does — the answer must not tell which emails exist.
	if response.UserId == uuid.Nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, authErrors.OtpCodeWrong))
		return
	}

	// Generate JWT tokens
	accessToken, err := uc.jwtService.GenerateAccessToken(response.UserId, response.SessionId, response.SystemRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	refreshToken, err := uc.jwtService.GenerateRefreshToken(response.UserId, response.SessionId, response.SystemRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildError(c, err))
		return
	}

	// Add tokens to response
	response.AccessToken = accessToken
	response.RefreshToken = refreshToken

	c.JSON(http.StatusOK, response)
}
