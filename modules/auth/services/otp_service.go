package services

import (
	"context"
	"errors"
	"fmt"
	"josex/web/modules/auth/config"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	otp "josex/web/modules/auth/services/otp"
	"math/rand"
	"time"
)

// OtpServiceImpl implements the OtpService interface
type OtpServiceImpl struct {
	otpRepository interfaces.OtpRepository
	providers     map[string]otp.OtpProvider
	config        *config.AuthConfig
	rng           *rand.Rand
}

// NewOtpService creates a new OTP service with provider registry
func NewOtpService(
	otpRepository interfaces.OtpRepository,
	providers map[string]otp.OtpProvider,
	config *config.AuthConfig,
) *OtpServiceImpl {
	return &OtpServiceImpl{
		otpRepository: otpRepository,
		providers:     providers,
		config:        config,
		rng:           rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// RequestOtp orchestrates OTP request process
func (s *OtpServiceImpl) RequestOtp(
	ctx context.Context,
	dto authModels.RequestOtpDto,
) (*authModels.RequestOtpResponse, error) {
	// Validate channel is enabled
	provider, exists := s.providers[dto.Channel]
	if !exists {
		return nil, errors.New("Invalid OTP channel: " + dto.Channel)
	}

	if !provider.IsEnabled() {
		return nil, errors.New("OTP channel is not enabled: " + dto.Channel)
	}

	// Generate 6-digit OTP code
	otpCode := fmt.Sprintf("%06d", s.rng.Intn(1000000))

	// Get language from context (set by LanguageMiddleware)
	// Default to "en" if not found
	lang := "en"
	if langValue := ctx.Value("lang"); langValue != nil {
		if langStr, ok := langValue.(string); ok {
			lang = langStr
		}
	}

	// Send OTP via provider
	expiresAt, err := provider.SendOtp(ctx, dto.Destination, otpCode, lang)
	if err != nil {
		return nil, fmt.Errorf("failed to send OTP via %s: %w", dto.Channel, err)
	}

	// Store OTP in database
	otpRecord, err := s.otpRepository.RequestOtp(
		ctx,
		dto.Destination,
		dto.Channel,
		otpCode,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to store OTP request: %w", err)
	}

	// Mask destination for security (show only last 2 digits)
	maskedDestination := maskDestination(dto.Destination)

	return &authModels.RequestOtpResponse{
		OtpId:       otpRecord.Id.String(),
		Destination: maskedDestination,
		Channel:     dto.Channel,
		ExpiresAt:   expiresAt,
		Message:     fmt.Sprintf("OTP sent to %s via %s", maskedDestination, dto.Channel),
	}, nil
}

// VerifyOtp orchestrates OTP verification process
func (s *OtpServiceImpl) VerifyOtp(
	ctx context.Context,
	dto authModels.VerifyOtpDto,
) (*authModels.VerifyOtpResponse, error) {
	// Validate channel exists
	_, exists := s.providers[dto.Channel]
	if !exists {
		return nil, errors.New("Invalid OTP channel: " + dto.Channel)
	}

	// Verify OTP in database
	sessionUser, err := s.otpRepository.VerifyOtp(ctx, dto)
	if err != nil {
		return nil, err
	}

	response := &authModels.VerifyOtpResponse{
		SessionId:  sessionUser.SessionId,
		UserId:     sessionUser.UserId,
		SystemRole: sessionUser.SystemRole,
	}

	return response, nil
}

// maskDestination masks a phone number or email for security
// Examples: +1234567890 → +1234****90, user@example.com → u****m@example.com
func maskDestination(destination string) string {
	if len(destination) <= 4 {
		return "***"
	}

	if destination[0] == '+' {
		// Phone number - mask middle digits
		return destination[:3] + "****" + destination[len(destination)-2:]
	} else if len(destination) > 5 {
		// Email - mask middle part
		atIndex := -1
		for i, c := range destination {
			if c == '@' {
				atIndex = i
				break
			}
		}

		if atIndex > 0 {
			return string(destination[0]) + "****" + string(destination[atIndex-1:])
		}
	}

	return "***"
}
