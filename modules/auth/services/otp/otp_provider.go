package otp

import (
	"context"
	"time"
)

// OtpProvider defines the interface for OTP delivery channels
// Each implementation (WhatsApp, SMS, Email) must satisfy this contract
type OtpProvider interface {
	// SendOtp sends the OTP code to the destination and returns expiration time
	// destination: phone number (e.g., "+1234567890") or email (e.g., "user@example.com")
	// otpCode: 6-digit OTP code (e.g., "123456")
	// lang: language code (e.g., "en", "es") for multilingual content
	// Returns: expiration time and error (if sending failed)
	SendOtp(ctx context.Context, destination string, otpCode string, lang string) (expiresAt time.Time, err error)

	// GetChannelName returns the unique channel identifier
	// Examples: "whatsapp", "sms", "email"
	GetChannelName() string

	// IsEnabled returns whether this provider is enabled/configured
	IsEnabled() bool
}
