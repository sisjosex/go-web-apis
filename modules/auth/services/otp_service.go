package services

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"josex/web/modules/auth/config"
	authErrors "josex/web/modules/auth/errors"
	"josex/web/modules/auth/interfaces"
	authModels "josex/web/modules/auth/models"
	otp "josex/web/modules/auth/services/otp"
	"log"
	"math/big"
)

// Errors the controller maps to a status; their text is the translation key the client receives.
var (
	// ErrChannelDisabled: no provider can carry the channel (and it cannot be relayed by email).
	ErrChannelDisabled = errors.New(authErrors.OtpChannelDisabled)
	// ErrProviderUnavailable: the provider failed to send; the stored code was invalidated.
	ErrProviderUnavailable = errors.New(authErrors.OtpProviderUnavailable)
)

// relayChannel carries phone channels while they have no provider of their own (AUTH-001 D1/D2).
const relayChannel = "email"

var phoneChannels = map[string]bool{"sms": true, "whatsapp": true}

// OtpServiceImpl implements the OtpService interface
type OtpServiceImpl struct {
	otpRepository interfaces.OtpRepository
	providers     map[string]otp.OtpProvider
	config        *config.AuthConfig
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
	}
}

// RequestOtp orchestrates OTP request process: pick the provider, store the code, send it.
// The SP validates destination and channel; a failed send invalidates the stored code.
func (s *OtpServiceImpl) RequestOtp(
	ctx context.Context,
	dto authModels.RequestOtpDto,
) (*authModels.RequestOtpResponse, error) {
	provider, relay, err := s.pickProvider(dto.Channel)
	if err != nil {
		return nil, err
	}

	otpCode, err := generateOtpCode()
	if err != nil {
		return nil, fmt.Errorf("generate OTP code: %w", err)
	}

	otpRecord, err := s.otpRepository.RequestOtp(ctx, dto.Destination, dto.Channel, otpCode, relay)
	if err != nil {
		return nil, err
	}

	sendTo := otpRecord.Destination
	if relay {
		sendTo = *otpRecord.RelayEmail
	}
	maskedDestination := maskDestination(sendTo)

	if err := provider.SendOtp(ctx, sendTo, otpCode, langFrom(ctx)); err != nil {
		log.Printf("OTP %s: sending via %s to %s failed: %v", otpRecord.Id, provider.GetChannelName(), maskedDestination, err)
		if invErr := s.otpRepository.InvalidateOtp(ctx, otpRecord.Id); invErr != nil {
			log.Printf("OTP %s: invalidating after a failed send: %v", otpRecord.Id, invErr)
		}
		return nil, ErrProviderUnavailable
	}

	return &authModels.RequestOtpResponse{
		OtpId:        otpRecord.Id.String(),
		Destination:  maskedDestination,
		Channel:      otpRecord.OtpChannel,
		DeliveredVia: provider.GetChannelName(),
		ExpiresAt:    otpRecord.ExpiresAt,
		Message:      fmt.Sprintf("OTP sent to %s via %s", maskedDestination, provider.GetChannelName()),
	}, nil
}

// pickProvider returns the channel's own provider, or the email provider for a phone channel that has
// none yet (relay = true).
func (s *OtpServiceImpl) pickProvider(channel string) (provider otp.OtpProvider, relay bool, err error) {
	if p, ok := s.providers[channel]; ok && p.IsEnabled() {
		return p, false, nil
	}
	if p, ok := s.providers[relayChannel]; ok && p.IsEnabled() && phoneChannels[channel] {
		return p, true, nil
	}
	return nil, false, ErrChannelDisabled
}

// VerifyOtp orchestrates OTP verification process; the SP matches destination, channel and code.
func (s *OtpServiceImpl) VerifyOtp(
	ctx context.Context,
	dto authModels.VerifyOtpDto,
) (*authModels.VerifyOtpResponse, error) {
	sessionUser, err := s.otpRepository.VerifyOtp(ctx, dto)
	if err != nil {
		return nil, err
	}

	return &authModels.VerifyOtpResponse{
		SessionId:  sessionUser.SessionId,
		UserId:     sessionUser.UserId,
		SystemRole: sessionUser.SystemRole,
	}, nil
}

// generateOtpCode returns a uniformly random 6-digit code from crypto/rand.
func generateOtpCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// langFrom reads the language set by LanguageMiddleware, defaulting to "en".
func langFrom(ctx context.Context) string {
	if lang, ok := ctx.Value("lang").(string); ok && lang != "" {
		return lang
	}
	return "en"
}

// maskDestination masks a phone number or email for security
// Examples: +1234567890 → +12****90, user@example.com → u****r@example.com
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
