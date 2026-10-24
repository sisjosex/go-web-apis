package controllers

import (
	"context"
	"errors"
	"net/http"

	billingErrors "josex/web/modules/billing/errors"
	billingModels "josex/web/modules/billing/models"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leebenson/conform"
)

// tenantID is the business the tenant chain resolved from X-Tenant-Slug.
func tenantID(c *gin.Context) (uuid.UUID, error) {
	raw, ok := c.Get("tenant_id")
	if !ok {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	id, err := uuid.Parse(raw.(string))
	if err != nil {
		return uuid.Nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	return id, nil
}

// GetTenantPlan is GET /tenant/plan: The business's plan.
func (ctrl *BillingController) GetTenantPlan(ctx context.Context, _ *api.None) (*api.Out[*billingModels.TenantPlan], error) {
	c := api.Gin(ctx)
	id, err := tenantID(c)
	if err != nil {
		return nil, err
	}
	plan, err := ctrl.billingService.GetTenantPlan(c.Request.Context(), id)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(plan)
}

// GetTenantUsage is GET /usage: The business's usage against its limits.
func (ctrl *BillingController) GetTenantUsage(ctx context.Context, _ *api.None) (*api.Out[*billingModels.UsageResponse], error) {
	c := api.Gin(ctx)
	id, err := tenantID(c)
	if err != nil {
		return nil, err
	}
	usage, err := ctrl.billingService.GetTenantUsage(c.Request.Context(), id)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(usage)
}

// NotifyPaymentInput is NotifyPayment's request.
type NotifyPaymentInput struct {
	JSON api.Body[billingModels.NotifyPaymentDto]
}

// NotifyPayment is POST /payments/notice: "Ya pagué".
func (ctrl *BillingController) NotifyPayment(ctx context.Context, in *NotifyPaymentInput) (*api.Out[*billingModels.TenantPayment], error) {
	c := api.Gin(ctx)
	id, err := tenantID(c)
	if err != nil {
		return nil, err
	}
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	var dto billingModels.NotifyPaymentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)
	payment, err := ctrl.billingService.NotifyPayment(c.Request.Context(), id, userID, &dto)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(payment)
}

// ListNotifiedPaymentsResponse is ListNotifiedPayments's answer.
type ListNotifiedPaymentsResponse struct {
	Payments []billingModels.NotifiedPayment `json:"payments"`
}

// ListNotifiedPayments is GET /payments/pending: Payments waiting for confirmation.
func (ctrl *BillingController) ListNotifiedPayments(ctx context.Context, _ *api.None) (*api.Out[ListNotifiedPaymentsResponse], error) {
	c := api.Gin(ctx)
	payments, err := ctrl.billingService.ListNotifiedPayments(c.Request.Context())
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(ListNotifiedPaymentsResponse{Payments: payments})
}

// ConfirmPaymentInput is ConfirmPayment's request.
type ConfirmPaymentInput struct {
	ID string `path:"id"`
}

// ConfirmPayment is POST /payments/{id}/confirm: Confirm a payment.
func (ctrl *BillingController) ConfirmPayment(ctx context.Context, in *ConfirmPaymentInput) (*api.Out[*billingModels.TenantPayment], error) {
	c := api.Gin(ctx)
	paymentID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	payment, err := ctrl.billingService.ConfirmPayment(c.Request.Context(), paymentID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Message {
			case billingErrors.PaymentNotFound:
				return nil, api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, billingErrors.PaymentNotFound))
			case billingErrors.PaymentNotPending:
				return nil, api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, billingErrors.PaymentNotPending))
			}
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(payment)
}

// paymentWriteError answers the codes a payment write raises; reports whether it wrote the response.
func paymentWriteError(c *gin.Context, err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return nil
	}
	switch pgErr.Message {
	case billingErrors.PaymentNotFound, billingErrors.TenantNotFound:
		return api.Fail(http.StatusNotFound, coreErrors.BuildErrorSingle(c, pgErr.Message))
	case billingErrors.PaymentNotPending:
		return api.Fail(http.StatusConflict, coreErrors.BuildErrorSingle(c, pgErr.Message))
	case billingErrors.AdjustReasonRequired:
		return api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, pgErr.Message))
	}
	return nil
}

// RejectPaymentInput is RejectPayment's request.
type RejectPaymentInput struct {
	ID   string `path:"id"`
	JSON api.Body[billingModels.RejectPaymentDto]
}

// RejectPayment is POST /payments/{id}/reject: Reject a payment.
func (ctrl *BillingController) RejectPayment(ctx context.Context, in *RejectPaymentInput) (*api.Out[*billingModels.TenantPayment], error) {
	c := api.Gin(ctx)
	paymentID, err := uuid.Parse(in.ID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	var dto billingModels.RejectPaymentDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)
	payment, err := ctrl.billingService.RejectPayment(c.Request.Context(), paymentID, dto.Reason, userID)
	if err != nil {
		if err := paymentWriteError(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(payment)
}

// ListTenantSubscriptionsInput is ListTenantSubscriptions's request.
type ListTenantSubscriptionsInput struct {
	Query api.Query[billingModels.ListTenantSubscriptionsQuery]
}

// ListTenantSubscriptions is GET /admin/subscriptions: Every business and its plan.
func (ctrl *BillingController) ListTenantSubscriptions(ctx context.Context, in *ListTenantSubscriptionsInput) (*api.Out[*billingModels.ListTenantSubscriptionsResponse], error) {
	c := api.Gin(ctx)
	var query billingModels.ListTenantSubscriptionsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}
	out, err := ctrl.billingService.ListTenantSubscriptions(c.Request.Context(), query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(out)
}

// AdjustTenantSubscriptionInput is AdjustTenantSubscription's request.
type AdjustTenantSubscriptionInput struct {
	BusinessID string `path:"business_id"`
	JSON       api.Body[billingModels.AdjustSubscriptionDto]
}

// AdjustTenantSubscription is PUT /admin/tenants/{business_id}/subscription: Set a business's plan by hand.
func (ctrl *BillingController) AdjustTenantSubscription(ctx context.Context, in *AdjustTenantSubscriptionInput) (*api.Out[*billingModels.TenantPlan], error) {
	c := api.Gin(ctx)
	// The business the platform acts on, named in the path — never the caller's own tenant.
	tenantID, err := uuid.Parse(in.BusinessID)
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, coreErrors.InvalidUUID))
	}
	userID, err := extractUserID(c)
	if err != nil {
		return nil, api.Fail(http.StatusUnauthorized, coreErrors.BuildErrorSingle(c, coreErrors.UserUnauthorized))
	}
	var dto billingModels.AdjustSubscriptionDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, billingErrors.BillingValidationFailed, utils.ExtractValidationError(c, err)))
	}
	_ = conform.Strings(&dto)
	plan, err := ctrl.billingService.AdjustTenantSubscription(c.Request.Context(), tenantID, &dto, userID)
	if err != nil {
		if err := paymentWriteError(c, err); err != nil {
			return nil, err
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return api.OK(plan)
}
