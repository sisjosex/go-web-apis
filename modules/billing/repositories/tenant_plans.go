package repositories

import (
	"context"
	"encoding/json"

	billingModels "josex/web/modules/billing/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Business plans live in the platform database (BILLING-001): they are read from the primary pool even
// inside a request routed to a dedicated tenant database. Only the usage counts are the tenant's.

// GetTenantPlan returns a business's plan, its limits now and the price list.
func (r *billingRepository) GetTenantPlan(ctx context.Context, tenantID uuid.UUID) (*billingModels.TenantPlan, error) {
	plan := &billingModels.TenantPlan{}
	var limits, prices []byte
	err := r.dbService.GetPrimaryPool().QueryRow(ctx, `SELECT * FROM billing.sp_get_tenant_plan($1)`, tenantID).
		Scan(&plan.Plan, &plan.Cycle, &plan.Status, &plan.PeriodEnd, &plan.EffectivePlan, &limits, &prices)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(limits, &plan.Limits); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(prices, &plan.Prices); err != nil {
		return nil, err
	}
	return plan, nil
}

func scanTenantPayment(row pgx.Row) (*billingModels.TenantPayment, error) {
	p := &billingModels.TenantPayment{}
	err := row.Scan(&p.ID, &p.TenantID, &p.UserID, &p.Plan, &p.Cycle, &p.Amount, &p.Currency, &p.Status,
		&p.Reference, &p.PaidAt, &p.PeriodStart, &p.PeriodEnd, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NotifyPayment records the customer's "Ya pagué" for the platform to confirm.
func (r *billingRepository) NotifyPayment(ctx context.Context, tenantID, userID uuid.UUID, dto *billingModels.NotifyPaymentDto) (*billingModels.TenantPayment, error) {
	return scanTenantPayment(r.dbService.GetPrimaryPool().QueryRow(ctx,
		`SELECT * FROM billing.sp_notify_payment($1, $2, $3, $4, $5)`, tenantID, userID, dto.Plan, dto.Cycle, dto.Reference))
}

// ConfirmPayment completes a notified payment and extends the business's plan by its cycle.
func (r *billingRepository) ConfirmPayment(ctx context.Context, paymentID, confirmedBy uuid.UUID) (*billingModels.TenantPayment, error) {
	return scanTenantPayment(r.dbService.GetPrimaryPool().QueryRow(ctx,
		`SELECT * FROM billing.sp_confirm_payment($1, $2)`, paymentID, confirmedBy))
}

// ListNotifiedPayments is the platform's queue, oldest first.
func (r *billingRepository) ListNotifiedPayments(ctx context.Context) ([]billingModels.NotifiedPayment, error) {
	rows, err := r.dbService.GetPrimaryPool().Query(ctx, `SELECT * FROM billing.sp_list_notified_payments()`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []billingModels.NotifiedPayment{}
	for rows.Next() {
		var p billingModels.NotifiedPayment
		if err := rows.Scan(&p.ID, &p.TenantID, &p.TenantName, &p.UserID, &p.Plan, &p.Cycle, &p.Amount,
			&p.Currency, &p.Reference, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ExpireSubscriptions moves the paid plans past their grace to expired; answers how many.
func (r *billingRepository) ExpireSubscriptions(ctx context.Context, graceDays int) (int, error) {
	var n int
	err := r.dbService.GetPrimaryPool().QueryRow(ctx, `SELECT billing.sp_expire_subscriptions($1)`, graceDays).Scan(&n)
	return n, err
}

// GetTenantUsage counts what the business holds: riders, vehicles and products from its own database
// (ctx is routed there), members from the platform's.
func (r *billingRepository) GetTenantUsage(ctx context.Context, tenantID uuid.UUID) (map[string]int64, error) {
	used := map[string]int64{}
	rows, err := r.dbService.Query(ctx, `SELECT * FROM public.sp_get_tenant_usage($1)`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var feature string
		var n int64
		if err := rows.Scan(&feature, &n); err != nil {
			return nil, err
		}
		used[feature] = n
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var members int64
	if err := r.dbService.GetPrimaryPool().QueryRow(ctx,
		`SELECT COUNT(*) FROM tenancy.tenant_users WHERE tenant_id = $1 AND is_active`, tenantID).Scan(&members); err != nil {
		return nil, err
	}
	used["users_per_tenant"] = members
	return used, nil
}
