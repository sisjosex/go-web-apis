package interfaces

import (
	"context"
	authModels "josex/web/modules/auth/models"
)

// OtpRepository defines the interface for OTP database operations
type OtpRepository interface {
	// RequestOtp stores a new OTP request in the database
	// Calls: auth.sp_request_otp(p_destination, p_otp_channel, p_otp_code)
	RequestOtp(ctx context.Context, destination string, channel string, otpCode string) (*authModels.OtpRequest, error)

	// VerifyOtp validates and verifies an OTP code
	// Calls: auth.sp_verify_otp(p_destination, p_otp_code, p_otp_channel, ...)
	// Returns session user if exists, or basic session info if user doesn't exist yet
	VerifyOtp(ctx context.Context, dto authModels.VerifyOtpDto) (*authModels.SessionUser, error)
}

// OtpService defines the interface for OTP business logic
type OtpService interface {
	// RequestOtp orchestrates the OTP request process:
	// 1. Validates destination and channel
	// 2. Generates a random 6-digit code
	// 3. Calls OTP provider to send (WhatsApp, SMS, Email)
	// 4. Stores OTP in database
	// Returns expiration time
	RequestOtp(ctx context.Context, dto authModels.RequestOtpDto) (*authModels.RequestOtpResponse, error)

	// VerifyOtp orchestrates the OTP verification process:
	// 1. Validates destination and OTP code
	// 2. Calls repository to verify against database
	// 3. If user exists, returns SessionUser with JWT-ready data
	// 4. If user doesn't exist, returns signup hint
	VerifyOtp(ctx context.Context, dto authModels.VerifyOtpDto) (*authModels.VerifyOtpResponse, error)
}
