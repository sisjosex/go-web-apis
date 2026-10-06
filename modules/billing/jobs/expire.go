// Package jobs holds billing's periodic work (BILLING-001 D3).
package jobs

import (
	"context"
	"log"

	billingInterfaces "josex/web/modules/billing/interfaces"

	"github.com/hibiken/asynq"
)

// TaskExpire moves the paid business plans past their end plus the grace back to the free limits.
const TaskExpire = "billing:expire"

// ExpireCron runs it once a day, before the working day in Bolivia (UTC-4).
const ExpireCron = "15 9 * * *"

// ExpireHandler runs the expiry over the platform database; it is idempotent.
func ExpireHandler(service billingInterfaces.BillingService) asynq.HandlerFunc {
	return func(ctx context.Context, _ *asynq.Task) error {
		n, err := service.ExpireSubscriptions(ctx)
		if err != nil {
			return err
		}
		if n > 0 {
			log.Printf("💳 billing: %d business plan(s) expired", n)
		}
		return nil
	}
}
