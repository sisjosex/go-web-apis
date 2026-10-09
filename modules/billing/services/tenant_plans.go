package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"josex/web/config"
	billingModels "josex/web/modules/billing/models"

	"github.com/google/uuid"
)

// planCacheTTL bounds how long a business's limits are reused (BILLING-001 D2): a confirmed payment
// drops the entry in this process at once; another replica sees it within the TTL.
const planCacheTTL = 60 * time.Second

// graceDays is how long a paid plan keeps its limits past its end (D3).
const graceDays = 7

type cachedPlan struct {
	limits map[string]int
	until  time.Time
}

// planCache is package-level so every billing service in the process shares and drops the same entries.
var planCache sync.Map // uuid.UUID → cachedPlan

func (s *billingService) GetTenantPlan(ctx context.Context, tenantID uuid.UUID) (*billingModels.TenantPlan, error) {
	plan, err := s.repo.GetTenantPlan(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	conf := config.ModularAppConfig.Billing
	plan.Payment = billingModels.PaymentInstructions{
		QRImageURL: conf.PaymentQRImageURL,
		Bank:       conf.PaymentBank,
		Account:    conf.PaymentAccount,
		Holder:     conf.PaymentHolder,
	}
	planCache.Store(tenantID, cachedPlan{limits: plan.Limits, until: time.Now().Add(planCacheTTL)})
	return plan, nil
}

// TenantLimit answers from the cache while fresh; a feature the plan does not name is unlimited.
func (s *billingService) TenantLimit(ctx context.Context, tenantID uuid.UUID, feature string) (int, error) {
	if entry, ok := planCache.Load(tenantID); ok {
		if cached := entry.(cachedPlan); time.Now().Before(cached.until) {
			return limitOf(cached.limits, feature), nil
		}
	}
	plan, err := s.GetTenantPlan(ctx, tenantID)
	if err != nil {
		return -1, err
	}
	return limitOf(plan.Limits, feature), nil
}

func limitOf(limits map[string]int, feature string) int {
	if limit, ok := limits[feature]; ok {
		return limit
	}
	return -1
}

// usageFeatures are the limits a business sees on its billing page, in this order.
var usageFeatures = []string{"tracking_riders", "tracking_vehicles", "inventory_items", "users_per_tenant"}

func (s *billingService) GetTenantUsage(ctx context.Context, tenantID uuid.UUID) (*billingModels.UsageResponse, error) {
	used, err := s.repo.GetTenantUsage(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := &billingModels.UsageResponse{Features: make([]billingModels.UsageFeature, 0, len(usageFeatures))}
	for _, feature := range usageFeatures {
		limit, err := s.TenantLimit(ctx, tenantID, feature)
		if err != nil {
			return nil, err
		}
		out.Features = append(out.Features, billingModels.UsageFeature{Feature: feature, Used: used[feature], Limit: limit})
	}
	return out, nil
}

func (s *billingService) NotifyPayment(ctx context.Context, tenantID, userID uuid.UUID, dto *billingModels.NotifyPaymentDto) (*billingModels.TenantPayment, error) {
	payment, err := s.repo.NotifyPayment(ctx, tenantID, userID, dto)
	if err != nil {
		return nil, err
	}
	if admin := config.ModularAppConfig.Billing.AdminEmail; admin != "" {
		s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, string, string) {
			return admin, "Pago por confirmar: " + deref(contact.TenantName),
				fmt.Sprintf("%s avisó un pago de %.2f %s (%s %s), código %s.\n\nConfírmalo o recházalo en Plataforma → Suscripciones.",
					deref(contact.TenantName), payment.Amount, payment.Currency, payment.Plan, payment.Cycle, deref(payment.Reference))
		})
	}
	return payment, nil
}

func (s *billingService) ConfirmPayment(ctx context.Context, paymentID, confirmedBy uuid.UUID) (*billingModels.TenantPayment, error) {
	payment, err := s.repo.ConfirmPayment(ctx, paymentID, confirmedBy)
	if err != nil {
		return nil, err
	}
	planCache.Delete(payment.TenantID)
	s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, string, string) {
		until := ""
		if payment.PeriodEnd != nil {
			until = payment.PeriodEnd.Format("02/01/2006")
		}
		return deref(contact.Email), "Tu pago fue confirmado",
			fmt.Sprintf("Hola %s,\n\nConfirmamos tu pago de %.2f %s. El plan %s de %s está activo hasta el %s.",
				deref(contact.FirstName), payment.Amount, payment.Currency, payment.Plan, deref(contact.TenantName), until)
	})
	return payment, nil
}

// RejectPayment turns a notice down and tells the customer why (BILLING-002 D3, D4).
func (s *billingService) RejectPayment(ctx context.Context, paymentID uuid.UUID, reason string, rejectedBy uuid.UUID) (*billingModels.TenantPayment, error) {
	payment, err := s.repo.RejectPayment(ctx, paymentID, reason, rejectedBy)
	if err != nil {
		return nil, err
	}
	s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, string, string) {
		return deref(contact.Email), "No pudimos confirmar tu pago",
			fmt.Sprintf("Hola %s,\n\nNo pudimos confirmar tu pago de %.2f %s para %s: %s\n\nSi ya pagaste, responde a este correo con el comprobante.",
				deref(contact.FirstName), payment.Amount, payment.Currency, deref(contact.TenantName), reason)
	})
	return payment, nil
}

func (s *billingService) ListTenantSubscriptions(ctx context.Context, query billingModels.ListTenantSubscriptionsQuery) (*billingModels.ListTenantSubscriptionsResponse, error) {
	return s.repo.ListTenantSubscriptions(ctx, query)
}

// AdjustTenantSubscription sets the plan by hand; the business's cached limits go at once.
func (s *billingService) AdjustTenantSubscription(ctx context.Context, tenantID uuid.UUID, dto *billingModels.AdjustSubscriptionDto, adjustedBy uuid.UUID) (*billingModels.TenantPlan, error) {
	plan, err := s.repo.AdjustTenantSubscription(ctx, tenantID, dto, adjustedBy)
	if err != nil {
		return nil, err
	}
	planCache.Delete(tenantID)
	return plan, nil
}

// tell emails one person about a payment, after the write and off the request (D4): one read for who to
// write to, and a mail that fails is logged, never the write's failure.
func (s *billingService) tell(ctx context.Context, payment *billingModels.TenantPayment, compose func(*billingModels.PaymentContact) (to, subject, body string)) {
	contact, err := s.repo.PaymentContact(ctx, payment.ID)
	if err != nil {
		log.Printf("⚠️  billing: contact for payment %s: %v", payment.ID, err)
		return
	}
	to, subject, body := compose(contact)
	if to == "" {
		return
	}
	go func() {
		if err := s.email.SendPlainEmail(to, subject, body); err != nil {
			log.Printf("⚠️  billing: email for payment %s: %v", payment.ID, err)
		}
	}()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *billingService) ListNotifiedPayments(ctx context.Context) ([]billingModels.NotifiedPayment, error) {
	return s.repo.ListNotifiedPayments(ctx)
}

// ExpireSubscriptions drops every cached plan when one expired: which tenants it touched is not
// returned, and the job runs once a day.
func (s *billingService) ExpireSubscriptions(ctx context.Context) (int, error) {
	n, err := s.repo.ExpireSubscriptions(ctx, graceDays)
	if err == nil && n > 0 {
		planCache.Range(func(key, _ any) bool {
			planCache.Delete(key)
			return true
		})
	}
	return n, err
}
