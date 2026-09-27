package otp

import (
	"context"
)

// OtpProvider defines the interface for OTP delivery channels.
// A channel is live once a provider for it is registered in routes/otp_routes.go and reports IsEnabled;
// until then the OTP service relays a phone channel (sms, whatsapp) by email.
type OtpProvider interface {
	// SendOtp delivers the code to the destination
	// destination: phone number (e.g., "+59170000000") or email (e.g., "user@example.com")
	// otpCode: 6-digit OTP code
	// lang: language code (e.g., "en", "es") for multilingual content
	SendOtp(ctx context.Context, destination string, otpCode string, lang string) error

	// GetChannelName returns the unique channel identifier
	// Examples: "whatsapp", "sms", "email"
	GetChannelName() string

	// IsEnabled returns whether this provider is configured to actually send
	IsEnabled() bool
}
