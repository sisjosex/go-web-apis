package otp

import (
	"context"
	"fmt"
	"josex/web/modules/auth/config"
	"time"
)

// SmsProvider implements OtpProvider for SMS delivery via Twilio
type SmsProvider struct {
	config    *config.AuthConfig
	isEnabled bool
}

// NewSmsProvider creates a new SMS OTP provider
func NewSmsProvider(authConfig *config.AuthConfig) *SmsProvider {
	return &SmsProvider{
		config:    authConfig,
		isEnabled: authConfig.TwilioAccountSID != "" && authConfig.TwilioSmsNumber != "",
	}
}

// SendOtp sends OTP via SMS using Twilio API
// For now, this is a mock implementation that logs to console
// In production, integrate with Twilio SDK: github.com/twilio/twilio-go
func (p *SmsProvider) SendOtp(ctx context.Context, destination string, otpCode string, lang string) (expiresAt time.Time, err error) {
	// TODO: Implement Twilio SMS API integration
	// Example code structure:
	/*
		client := twilio.NewRestClientWithParams(p.config.TwilioAccountSID, p.config.TwilioAuthToken, twilio.ClientParams{})

		params := &openapi.CreateMessageParams{}
		params.SetTo(destination)
		params.SetFrom(p.config.TwilioSmsNumber)
		params.SetBody(fmt.Sprintf("Your verification code is: %s. Valid for 10 minutes.", otpCode))

		resp, err := client.Api.CreateMessage(params)
		if err != nil {
			return time.Time{}, err
		}

		expiresAt = time.Now().Add(10 * time.Minute)
		return expiresAt, nil
	*/

	// Mock implementation - logs to console
	fmt.Printf("💬 [MOCK] SMS OTP sent to %s: %s (lang=%s, expires in 10 minutes)\n", destination, otpCode, lang)
	expiresAt = time.Now().Add(10 * time.Minute)
	return expiresAt, nil
}

// GetChannelName returns the channel identifier
func (p *SmsProvider) GetChannelName() string {
	return "sms"
}

// IsEnabled returns whether the provider is configured and enabled
func (p *SmsProvider) IsEnabled() bool {
	return p.isEnabled
}
