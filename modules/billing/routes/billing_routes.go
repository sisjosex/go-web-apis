package routes

import (
	"josex/web/modules/billing/controllers"
	authServices "josex/web/modules/auth/services"

	"github.com/gin-gonic/gin"
)

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
		billing.PUT("/subscription", ctrl.UpsertSubscription)

		// Payment history
		billing.GET("/payments", ctrl.ListPayments)
		billing.POST("/payments", ctrl.RecordPayment)
	}
}
