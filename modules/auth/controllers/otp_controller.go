package controllers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	"josex/web/modules/auth/services"
	"josex/web/modules/core/api"
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

// RequestOtpInput is the three OTP request handlers' request.
type RequestOtpInput struct {
	JSON api.Body[authModels.RequestOtpRequestDto]
}

// RequestOtpWhatsApp sends a code by WhatsApp (relayed by email while it has no provider).
func (uc *OtpController) RequestOtpWhatsApp(ctx context.Context, in *RequestOtpInput) (*api.Out[*authModels.RequestOtpResponse], error) {
	return uc.requestOtp(ctx, in, "whatsapp")
}

// RequestOtpSms sends a code by SMS (relayed by email while it has no provider).
func (uc *OtpController) RequestOtpSms(ctx context.Context, in *RequestOtpInput) (*api.Out[*authModels.RequestOtpResponse], error) {
	return uc.requestOtp(ctx, in, "sms")
}

// RequestOtpEmail sends a code by email.
func (uc *OtpController) RequestOtpEmail(ctx context.Context, in *RequestOtpInput) (*api.Out[*authModels.RequestOtpResponse], error) {
	return uc.requestOtp(ctx, in, "email")
}

// requestOtp is the body of the three request handlers; the route names the channel.
func (uc *OtpController) requestOtp(ctx context.Context, in *RequestOtpInput, channel string) (*api.Out[*authModels.RequestOtpResponse], error) {
	c := api.Gin(ctx)
	var requestDto authModels.RequestOtpRequestDto

	if err := in.JSON.Into(ctx, &requestDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpRequestFailed, coreUtils.ExtractValidationError(c, err)))
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
		return nil, api.Fail(status, body)
	}

	return api.OK(response)
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

// VerifyOtpInput is VerifyOtp's request.
type VerifyOtpInput struct {
	JSON api.Body[authModels.VerifyOtpRequestDto]
}

// VerifyOtp is POST /otp/verify: Verify OTP code.
func (uc *OtpController) VerifyOtp(ctx context.Context, in *VerifyOtpInput) (*api.Out[*authModels.VerifyOtpResponse], error) {
	c := api.Gin(ctx)
	var verifyDto authModels.VerifyOtpRequestDto

	if err := in.JSON.Into(ctx, &verifyDto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, authErrors.OtpVerifyFailed, coreUtils.ExtractValidationError(c, err)))
	}

	declaredClient, err := clientType(c)
	if err != nil {
		return nil, err
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
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildError(c, err))
	}
	// A right code for an email with no active account opens nothing, and says so the way a wrong
	// code does — the answer must not tell which emails exist.
	if response.UserId == uuid.Nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, authErrors.OtpCodeWrong))
	}

	// Generate JWT tokens
	accessToken, err := uc.jwtService.GenerateAccessToken(response.UserId, response.SessionId, response.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	refreshToken, err := uc.jwtService.GenerateRefreshToken(response.UserId, response.SessionId, response.SystemRole)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}

	// Add tokens to response
	response.AccessToken = accessToken
	response.RefreshToken = refreshToken

	return api.OK(response)
}
