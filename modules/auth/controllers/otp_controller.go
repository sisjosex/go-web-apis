package controllers

import (
	"net/http"
	"strings"

	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	coreErrors "josex/web/modules/core/errors"
	coreUtils "josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
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
// @Summary Request OTP via WhatsApp
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Validation error or channel disabled"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/whatsapp/request [post]
func (uc *OtpController) RequestOtpWhatsApp(c *gin.Context) {
	var requestDto authModels.RequestOtpRequestDto

	if err := c.ShouldBindJSON(&requestDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpRequestFailed, coreUtils.ExtractValidationError(c, err)))
		return
	}

	// Force channel to whatsapp
	requestDto.Channel = "whatsapp"

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
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  deviceInfo,
		DeviceOs:    deviceOs,
		Browser:     browser,
		UserAgent:   userAgent,
	}

	response, err := uc.otpService.RequestOtp(c.Request.Context(), otpDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RequestOtpSms godoc
// @Summary Request OTP via SMS
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Validation error or channel disabled"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/sms/request [post]
func (uc *OtpController) RequestOtpSms(c *gin.Context) {
	var requestDto authModels.RequestOtpRequestDto

	if err := c.ShouldBindJSON(&requestDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpRequestFailed, coreUtils.ExtractValidationError(c, err)))
		return
	}

	// Force channel to sms
	requestDto.Channel = "sms"

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
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  deviceInfo,
		DeviceOs:    deviceOs,
		Browser:     browser,
		UserAgent:   userAgent,
	}

	response, err := uc.otpService.RequestOtp(c.Request.Context(), otpDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
}

// RequestOtpEmail godoc
// @Summary Request OTP via Email
// @Tags Auth - OTP
// @Accept  json
// @Produce json
// @Param   request body authModels.RequestOtpRequestDto true "Request OTP"
// @Success 200 {object} authModels.RequestOtpResponse
// @Failure 400 {object} object "Validation error or channel disabled"
// @Failure 500 {object} object "Server error"
// @Router  /auth/otp/email/request [post]
func (uc *OtpController) RequestOtpEmail(c *gin.Context) {
	var requestDto authModels.RequestOtpRequestDto

	if err := c.ShouldBindJSON(&requestDto); err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpRequestFailed, coreUtils.ExtractValidationError(c, err)))
		return
	}

	// Force channel to email
	requestDto.Channel = "email"

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
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  deviceInfo,
		DeviceOs:    deviceOs,
		Browser:     browser,
		UserAgent:   userAgent,
	}

	response, err := uc.otpService.RequestOtp(c.Request.Context(), otpDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	c.JSON(http.StatusOK, response)
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
	}

	response, err := uc.otpService.VerifyOtp(c.Request.Context(), otpVerifyDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
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
