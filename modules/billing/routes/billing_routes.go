package routes

import (
	"net/http"

	authServices "josex/web/modules/auth/services"
	"josex/web/modules/billing/controllers"
	billingErrors "josex/web/modules/billing/errors"
	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"

	"github.com/gin-gonic/gin"
)

// platformOnly lets only a super_admin write a plan or a payment (APP-009 D4). Until a checkout
// confirms a payment, a plan change is the platform's to make, after the customer has paid.
func platformOnly(c *gin.Context) {
	if role, _ := c.Get("system_role"); role != coreModels.SystemRoleSuperAdmin {
		c.AbortWithStatusJSON(http.StatusForbidden, coreErrors.BuildErrorSingle(c, billingErrors.PlanContactSales))
		return
	}
	c.Next()
}

// RegisterBillingRoutes mounts the billing endpoints under /billing.
// All routes require JWT authentication.
func RegisterBillingRoutes(
	rg *gin.RouterGroup,
	ctrl *controllers.BillingController,
	authMiddleware gin.HandlerFunc,
	jwtService authServices.JWTService,
) {
	_ = jwtService // reserved for future token-scoped checks

	billing := rg.Group("/billing")
	billing.Use(authMiddleware)
	{
		// Plan info (plan + all feature limits)
		api.Get(billing, "/plan", ctrl.GetPlanInfo, api.Doc{Summary: "Get current plan info and feature limits", Description: "Returns the authenticated user's active plan, subscription status, expiry date, and all feature limits.", Errors: []int{401}})

		// Subscription management
		api.Get(billing, "/subscription", ctrl.GetSubscription, api.Doc{Summary: "Get current subscription", Description: "The authenticated user's active subscription (billing.Subscription), or {\"subscription\": null} when there is none.", Errors: []int{401}})
		api.Put(billing, "/subscription", ctrl.UpsertSubscription, api.Doc{Summary: "Create or upgrade a subscription", Description: "Creates a new subscription or upgrades the existing one. Cancels the old subscription when switching plans.", Errors: []int{400, 401}}, platformOnly)

		// Payment history
		api.Get(billing, "/payments", ctrl.ListPayments, api.Doc{Summary: "List payment history", Description: "Returns paginated payment history for the authenticated user.", Errors: []int{401}})
		api.Post(billing, "/payments", ctrl.RecordPayment, api.Doc{Summary: "Record a payment", Description: "Records a payment against an active subscription.", Status: 201, Errors: []int{400, 401}}, platformOnly)

		// The platform's confirmation queue (BILLING-001 D3).
		api.Get(billing, "/payments/pending", ctrl.ListNotifiedPayments, api.Doc{Summary: "Payments waiting for confirmation", Description: "The platform's queue of notified payments, oldest first; super_admin only."}, platformOnly)
		api.Post(billing, "/payments/{id}/confirm", ctrl.ConfirmPayment, api.Doc{Summary: "Confirm a payment", Description: "Completes a notified payment and extends the business's plan one cycle from the later of now and its end (BILLING-001 D3); super_admin only.", Errors: []int{404, 409}}, platformOnly)
		api.Post(billing, "/payments/{id}/reject", ctrl.RejectPayment, api.Doc{Summary: "Reject a payment", Description: "Turns a notified payment down with the reason the customer is emailed; the plan does not change (BILLING-002 D3); super_admin only.", Errors: []int{404, 409}}, platformOnly)

		// The platform's businesses and the hand adjustment (BILLING-002 D3).
		api.Get(billing, "/admin/subscriptions", ctrl.ListTenantSubscriptions, api.Doc{Summary: "Every business and its plan", Description: "The platform's businesses list: plan, cycle, status, end and the notice under review, searched by name or code (BILLING-002 D3); super_admin only."}, platformOnly)
		api.Put(billing, "/admin/tenants/{business_id}/subscription", ctrl.AdjustTenantSubscription, api.Doc{Summary: "Set a business's plan by hand", Description: "Plan, cycle, status and end with a required reason; recorded as an adjustment with before and after (BILLING-002 D3); super_admin only.", Errors: []int{404}}, platformOnly)
	}
}

// RegisterTenantBillingRoutes mounts a business's own plan behind the tenant chain (BILLING-001): its
// plan and usage for every member, the payment notice for whoever holds the workspace.
func RegisterTenantBillingRoutes(rg *gin.RouterGroup, ctrl *controllers.BillingController, authMiddleware, tenant gin.HandlerFunc) {
	billing := rg.Group("/billing", authMiddleware, tenant)
	api.Get(billing, "/tenant/plan", ctrl.GetTenantPlan, api.Doc{Summary: "The business's plan", Description: "Plan, cycle, status and end of the X-Tenant-Slug business, the limits it has now (the free plan's once expired), the price list and where to pay (BILLING-001)."})
	api.Get(billing, "/usage", ctrl.GetTenantUsage, api.Doc{Summary: "The business's usage against its limits", Description: "Riders, vehicles, products and members the business holds, each with its plan limit (-1 unlimited)."})
	api.Post(billing, "/payments/notice", ctrl.NotifyPayment, api.Doc{Summary: "\"Ya pagué\"", Description: "Records the business's bank QR payment for a plan and cycle, for the platform to confirm (BILLING-001 D3).", Status: 201})
}
