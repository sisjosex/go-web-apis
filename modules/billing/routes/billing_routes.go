package routes

import (
	"net/http"

	authServices "josex/web/modules/auth/services"
	"josex/web/modules/billing/controllers"
	billingErrors "josex/web/modules/billing/errors"
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
		billing.GET("/plan", ctrl.GetPlanInfo)

		// Subscription management
		billing.GET("/subscription", ctrl.GetSubscription)
		billing.PUT("/subscription", platformOnly, ctrl.UpsertSubscription)

		// Payment history
		billing.GET("/payments", ctrl.ListPayments)
		billing.POST("/payments", platformOnly, ctrl.RecordPayment)
	}
}
