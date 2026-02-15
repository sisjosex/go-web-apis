package controllers

import (
	"context"
	"net/http"
	"strings"

	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	coreErrors "josex/web/modules/core/errors"
	coreUtils "josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/ua-parser/uap-go/uaparser"
)

// OtpController handles OTP-related requests
type OtpController struct {
	otpService interfaces.OtpService
	parser     *uaparser.Parser
}

// NewOtpController creates a new OTP controller
func NewOtpController(
	otpService interfaces.OtpService,
	parser *uaparser.Parser,
) *OtpController {
	return &OtpController{
		otpService: otpService,
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
	client := uc.parser.Parse(userAgent)

	otpDto := authModels.RequestOtpDto{
		Destination: requestDto.Destination,
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  strings.TrimSpace(client.Device.Family),
		DeviceOs:    strings.TrimSpace(client.Os.Family),
		Browser:     strings.TrimSpace(client.UserAgent.Family),
		UserAgent:   userAgent,
	}

	// Create context with language for multilingual email support
	ctx := c.Request.Context()
	if lang, exists := c.Get("lang"); exists {
		ctx = context.WithValue(ctx, "lang", lang)
	}

	response, err := uc.otpService.RequestOtp(ctx, otpDto)
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
	client := uc.parser.Parse(userAgent)

	otpDto := authModels.RequestOtpDto{
		Destination: requestDto.Destination,
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  strings.TrimSpace(client.Device.Family),
		DeviceOs:    strings.TrimSpace(client.Os.Family),
		Browser:     strings.TrimSpace(client.UserAgent.Family),
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
	client := uc.parser.Parse(userAgent)

	otpDto := authModels.RequestOtpDto{
		Destination: requestDto.Destination,
		Channel:     requestDto.Channel,
		DeviceId:    requestDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  strings.TrimSpace(client.Device.Family),
		DeviceOs:    strings.TrimSpace(client.Os.Family),
		Browser:     strings.TrimSpace(client.UserAgent.Family),
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
	client := uc.parser.Parse(userAgent)

	otpVerifyDto := authModels.VerifyOtpDto{
		Destination: verifyDto.Destination,
		OtpCode:     verifyDto.OtpCode,
		Channel:     verifyDto.Channel,
		DeviceId:    verifyDto.DeviceId,
		IpAddress:   coreUtils.GetClientIp(c),
		DeviceInfo:  strings.TrimSpace(client.Device.Family),
		DeviceOs:    strings.TrimSpace(client.Os.Family),
		Browser:     strings.TrimSpace(client.UserAgent.Family),
		UserAgent:   userAgent,
	}

	response, err := uc.otpService.VerifyOtp(c.Request.Context(), otpVerifyDto)
	if err != nil {
		c.JSON(http.StatusBadRequest, coreErrors.BuildError(c, err))
		return
	}

	// If user doesn't exist, return 200 OK with user_exists=false
	// Client should then proceed to registration
	if !response.UserExists {
		c.JSON(http.StatusOK, response)
		return
	}

	// If user exists, return JWT tokens
	// TODO: Generate JWT tokens using JWTService
	// For now, just return the session info
	c.JSON(http.StatusOK, response)
}
