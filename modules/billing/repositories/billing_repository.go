package repositories

import (
	"context"
	"errors"

	billingInterfaces "josex/web/modules/billing/interfaces"
	billingModels "josex/web/modules/billing/models"
	coreServices "josex/web/modules/core/services"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type billingRepository struct {
	dbService coreServices.DatabaseService
}

// NewBillingRepository constructs a new BillingRepository.
func NewBillingRepository(dbService coreServices.DatabaseService) billingInterfaces.BillingRepository {
	return &billingRepository{dbService: dbService}
}

// GetSubscription returns the most recent subscription for a user.
func (r *billingRepository) GetSubscription(ctx context.Context, userID uuid.UUID) (*billingModels.Subscription, error) {
	query := `SELECT * FROM billing.sp_get_subscription(p_user_id := $1)`
	row := r.dbService.QueryRow(ctx, query, userID)

	sub := &billingModels.Subscription{}
	err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.Plan,
		&sub.Status,
		&sub.StartedAt,
		&sub.ExpiresAt,
		&sub.CanceledAt,
		&sub.StripeSubID,
		&sub.StripeCustomerID,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		// pgx returns pgconn.PgError for SP-raised exceptions
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		// No rows → no subscription yet
		return nil, nil
	}

	return sub, nil
}

// UpsertSubscription creates or updates the active subscription.
func (r *billingRepository) UpsertSubscription(ctx context.Context, userID uuid.UUID, dto *billingModels.UpsertSubscriptionDto) (*billingModels.Subscription, error) {
	status := dto.Status
	if status == "" {
		status = "active"
	}

	query := `SELECT * FROM billing.sp_upsert_subscription(
		p_user_id            := $1,
		p_plan               := $2,
		p_status             := $3,
		p_expires_at         := $4,
		p_stripe_sub_id      := $5,
		p_stripe_customer_id := $6
	)`

	row := r.dbService.QueryRow(ctx, query,
		userID,
		dto.Plan,
		status,
		dto.ExpiresAt,
		dto.StripeSubID,
		dto.StripeCustomerID,
	)

	sub := &billingModels.Subscription{}
	err := row.Scan(
		&sub.ID,
		&sub.UserID,
		&sub.Plan,
		&sub.Status,
		&sub.StartedAt,
		&sub.ExpiresAt,
		&sub.CanceledAt,
		&sub.StripeSubID,
		&sub.StripeCustomerID,
		&sub.CreatedAt,
		&sub.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return sub, nil
}

// GetPlanInfo returns the resolved plan and all feature limits for a user.
func (r *billingRepository) GetPlanInfo(ctx context.Context, userID uuid.UUID) (*billingModels.PlanInfo, error) {
	query := `SELECT * FROM billing.sp_get_plan_info(p_user_id := $1)`
	rows, err := r.dbService.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	info := &billingModels.PlanInfo{}
	for rows.Next() {
		var feature string
		var limitValue int
		if err := rows.Scan(
			&info.Plan,
			&info.Status,
			&info.ExpiresAt,
			&feature,
			&limitValue,
		); err != nil {
			return nil, err
		}
		info.Limits = append(info.Limits, billingModels.PlanLimit{
			Feature:    feature,
			LimitValue: limitValue,
		})
	}

	if info.Plan == "" {
		info.Plan = "free"
		info.Status = "none"
	}

	info.IsActive = info.Status == "active" || info.Status == "trial"
	info.IsExpired = info.Status == "expired"

	return info, rows.Err()
}

// GetFeatureLimit returns the limit for a feature based on the user's active plan.
func (r *billingRepository) GetFeatureLimit(ctx context.Context, userID uuid.UUID, feature string) (int, error) {
	info, err := r.GetPlanInfo(ctx, userID)
	if err != nil {
		return 0, err
	}

	for _, l := range info.Limits {
		if l.Feature == feature {
			return l.LimitValue, nil
		}
	}

	return -1, nil // feature not configured → unlimited
}

// RecordPayment inserts a payment record.
func (r *billingRepository) RecordPayment(ctx context.Context, userID uuid.UUID, dto *billingModels.RecordPaymentDto) (*billingModels.Payment, error) {
	status := dto.Status
	if status == "" {
		status = "completed"
	}

	query := `SELECT * FROM billing.sp_record_payment(
		p_subscription_id     := $1,
		p_user_id             := $2,
		p_amount              := $3,
		p_currency            := $4,
		p_status              := $5,
		p_provider            := $6,
		p_provider_payment_id := $7,
		p_period_start        := $8,
		p_period_end          := $9
	)`

	currency := dto.Currency
	if currency == "" {
		currency = "USD"
	}

	row := r.dbService.QueryRow(ctx, query,
		dto.SubscriptionID,
		userID,
		dto.Amount,
		currency,
		status,
		dto.Provider,
		dto.ProviderPaymentID,
		dto.PeriodStart,
		dto.PeriodEnd,
	)

	p := &billingModels.Payment{}
	var scannedUserID uuid.UUID // sp_record_payment returns user_id; scan but discard
	err := row.Scan(
		&p.ID,
		&p.SubscriptionID,
		&scannedUserID,
		&p.Amount,
		&p.Currency,
		&p.Status,
		&p.Provider,
		&p.ProviderPaymentID,
		&p.PaidAt,
		&p.PeriodStart,
		&p.PeriodEnd,
		&p.CreatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return nil, pgErr
		}
		return nil, err
	}

	return p, nil
}

// ListPayments returns paginated payment history for a user.
func (r *billingRepository) ListPayments(ctx context.Context, userID uuid.UUID, page, limit int) (*billingModels.PaymentListResponse, error) {
	query := `SELECT * FROM billing.sp_list_payments(p_user_id := $1, p_page := $2, p_limit := $3)`
	rows, err := r.dbService.Query(ctx, query, userID, page, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	resp := &billingModels.PaymentListResponse{
		Page:  page,
		Limit: limit,
	}

	for rows.Next() {
		p := billingModels.Payment{}
		if err := rows.Scan(
			&p.ID,
			&p.SubscriptionID,
			&p.Amount,
			&p.Currency,
			&p.Status,
			&p.Provider,
			&p.ProviderPaymentID,
			&p.PaidAt,
			&p.PeriodStart,
			&p.PeriodEnd,
			&p.CreatedAt,
			&resp.TotalCount,
		); err != nil {
			return nil, err
		}
		resp.Data = append(resp.Data, p)
	}

	if resp.Data == nil {
		resp.Data = []billingModels.Payment{}
	}

	return resp, rows.Err()
}
