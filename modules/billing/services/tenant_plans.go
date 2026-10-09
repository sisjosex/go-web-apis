package services

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"josex/web/config"
	billingModels "josex/web/modules/billing/models"
	coreServices "josex/web/modules/core/services"

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
		s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, paymentMail) {
			return admin, paymentMail{
				Title:    "Pago por confirmar: " + deref(contact.TenantName),
				Greeting: "Hola,",
				Message:  deref(contact.TenantName) + " avisó un pago. Confírmalo o recházalo en Plataforma → Suscripciones.",
				Details: append(paymentDetails(payment),
					mailDetail{"Plan", payment.Plan + " · " + payment.Cycle},
					mailDetail{"Código", deref(payment.Reference)}),
				ButtonText: "Abrir suscripciones",
				Path:       "/platform/subscriptions",
			}
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
	s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, paymentMail) {
		details := append(paymentDetails(payment), mailDetail{"Plan", payment.Plan + " · " + deref(contact.TenantName)})
		if payment.PeriodEnd != nil {
			details = append(details, mailDetail{"Activo hasta", payment.PeriodEnd.Format("02/01/2006")})
		}
		return deref(contact.Email), paymentMail{
			Title:      "Tu pago fue confirmado",
			Greeting:   greeting(contact.FirstName),
			Message:    "Confirmamos tu pago. Tu plan ya está activo.",
			Details:    details,
			ButtonText: "Ver mi plan",
			Path:       "/billing",
		}
	})
	return payment, nil
}

// RejectPayment turns a notice down and tells the customer why (BILLING-002 D3, D4).
func (s *billingService) RejectPayment(ctx context.Context, paymentID uuid.UUID, reason string, rejectedBy uuid.UUID) (*billingModels.TenantPayment, error) {
	payment, err := s.repo.RejectPayment(ctx, paymentID, reason, rejectedBy)
	if err != nil {
		return nil, err
	}
	s.tell(ctx, payment, func(contact *billingModels.PaymentContact) (string, paymentMail) {
		return deref(contact.Email), paymentMail{
			Title:    "No pudimos confirmar tu pago",
			Greeting: greeting(contact.FirstName),
			Message:  "No pudimos confirmar tu pago para " + deref(contact.TenantName) + ". Si ya pagaste, responde a este correo con el comprobante.",
			Details:  append(paymentDetails(payment), mailDetail{"Motivo", reason}),
			// The customer is asked to answer with the receipt: no "do not reply" under this one.
			Replyable:  true,
			ButtonText: "Ver mi plan",
			Path:       "/billing",
		}
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

// paymentMail is one payment email: the shared layout of the other emails (title, greeting, a line, the
// payment's details as a table, a button into the app, the sign-off).
type paymentMail struct {
	Title, Greeting, Message string
	Details                  []mailDetail
	ButtonText, Path         string
	Replyable                bool
}

type mailDetail struct{ Label, Value string }

// paymentDetails is the amount every payment email starts its table with.
func paymentDetails(payment *billingModels.TenantPayment) []mailDetail {
	return []mailDetail{{"Monto", fmt.Sprintf("%.2f %s", payment.Amount, payment.Currency)}}
}

func greeting(firstName *string) string {
	if name := deref(firstName); name != "" {
		return "Hola " + name + ","
	}
	return "Hola,"
}

// tell emails one person about a payment, after the write and off the request (D4): one read for who to
// write to, and a mail that fails is logged, never the write's failure.
func (s *billingService) tell(ctx context.Context, payment *billingModels.TenantPayment, compose func(*billingModels.PaymentContact) (string, paymentMail)) {
	contact, err := s.repo.PaymentContact(ctx, payment.ID)
	if err != nil {
		log.Printf("⚠️  billing: contact for payment %s: %v", payment.ID, err)
		return
	}
	to, mail := compose(contact)
	if to == "" {
		return
	}
	core := config.ModularAppConfig.Core
	data := map[string]interface{}{
		"Title":      mail.Title,
		"Greeting":   mail.Greeting,
		"Message":    mail.Message,
		"Details":    mail.Details,
		"ButtonText": mail.ButtonText,
		"LandingURL": core.FrontendURL + mail.Path,
		"SignOff":    "El equipo de " + core.AppName,
	}
	if !mail.Replyable {
		data["AutomatedNote"] = "Este es un mensaje automático. Por favor, no respondas."
	}
	templatePath := coreServices.GetTemplatePath("billing", "payment-notice.html")
	go func() {
		if err := s.email.SendEmail(to, mail.Title, templatePath, data); err != nil {
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
