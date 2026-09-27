package repositories

import (
	"context"
	authModels "josex/web/modules/auth/models"
	coreServices "josex/web/modules/core/services"

	"github.com/google/uuid"
)

// OtpRepository implements the OtpRepository interface
type OtpRepository struct {
	dbService coreServices.DatabaseService
}

// NewOtpRepository creates a new OTP repository
func NewOtpRepository(dbService coreServices.DatabaseService) *OtpRepository {
	return &OtpRepository{
		dbService: dbService,
	}
}

// RequestOtp stores a new OTP request in the database
func (r *OtpRepository) RequestOtp(
	ctx context.Context,
	destination string,
	channel string,
	otpCode string,
	relayToEmail bool,
) (*authModels.OtpRequest, error) {
	otpRecord := &authModels.OtpRequest{}

	query := `
		SELECT * FROM auth.sp_request_otp($1, $2, $3, $4)
	`

	row := r.dbService.QueryRow(ctx, query, destination, channel, otpCode, relayToEmail)

	err := row.Scan(
		&otpRecord.Id,
		&otpRecord.Destination,
		&otpRecord.OtpChannel,
		&otpRecord.ExpiresAt,
		&otpRecord.RelayEmail,
	)

	if err != nil {
		return nil, err
	}

	return otpRecord, nil
}

// InvalidateOtp closes a stored code whose delivery failed
func (r *OtpRepository) InvalidateOtp(ctx context.Context, otpId uuid.UUID) error {
	_, err := r.dbService.Execute(ctx, `SELECT auth.sp_invalidate_otp($1)`, otpId)
	return err
}

// VerifyOtp verifies an OTP code against the database
// Returns session user if user exists, otherwise returns session-like object with user_exists=false
func (r *OtpRepository) VerifyOtp(
	ctx context.Context,
	dto authModels.VerifyOtpDto,
) (*authModels.SessionUser, error) {
	token := &authModels.SessionUser{}

	query := `
		SELECT * FROM auth.sp_verify_otp(
			p_destination := $1,
			p_otp_code := $2,
			p_otp_channel := $3,
			p_device_id := $4,
			p_device_info := $5,
			p_device_os := $6,
			p_browser := $7,
			p_ip_address := $8,
			p_user_agent := $9,
			p_client_type := $10
		)
	`

	params := []any{
		dto.Destination,
		dto.OtpCode,
		dto.Channel,
		dto.DeviceId,
		dto.DeviceInfo,
		dto.DeviceOs,
		dto.Browser,
		dto.IpAddress,
		dto.UserAgent,
		dto.ClientType,
	}

	row := r.dbService.QueryRow(ctx, query, params...)

	var sessionId uuid.UUID
	var userId *uuid.UUID
	var userExists bool
	var systemRole string
	var subscriptionPlan string

	err := row.Scan(
		&sessionId,
		&userId,
		&userExists,
		&systemRole,
		&subscriptionPlan,
	)

	if err != nil {
		return nil, err
	}

	token.SessionId = sessionId
	if userId != nil {
		token.UserId = *userId
		token.SystemRole = systemRole
		token.SubscriptionPlan = subscriptionPlan
	}

	return token, nil
}
