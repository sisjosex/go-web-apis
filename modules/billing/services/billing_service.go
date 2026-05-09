package services

import (
	"context"

	billingInterfaces "josex/web/modules/billing/interfaces"
	billingModels "josex/web/modules/billing/models"

	"github.com/google/uuid"
)

type billingService struct {
	repo billingInterfaces.BillingRepository
}

// NewBillingService constructs a new BillingService.
func NewBillingService(repo billingInterfaces.BillingRepository) billingInterfaces.BillingService {
	return &billingService{repo: repo}
}

func (s *billingService) GetSubscription(ctx context.Context, userID uuid.UUID) (*billingModels.Subscription, error) {
	return s.repo.GetSubscription(ctx, userID)
}

func (s *billingService) UpsertSubscription(ctx context.Context, userID uuid.UUID, dto *billingModels.UpsertSubscriptionDto) (*billingModels.Subscription, error) {
	return s.repo.UpsertSubscription(ctx, userID, dto)
}

func (s *billingService) GetPlanInfo(ctx context.Context, userID uuid.UUID) (*billingModels.PlanInfo, error) {
	return s.repo.GetPlanInfo(ctx, userID)
}

func (s *billingService) GetFeatureLimit(ctx context.Context, userID uuid.UUID, feature string) (int, error) {
	return s.repo.GetFeatureLimit(ctx, userID, feature)
}

func (s *billingService) IsLimitReached(ctx context.Context, userID uuid.UUID, feature string, currentCount int) (bool, error) {
	limit, err := s.repo.GetFeatureLimit(ctx, userID, feature)
	if err != nil {
		return false, err
	}
	if limit == -1 {
		return false, nil // unlimited
	}
	return currentCount >= limit, nil
}

func (s *billingService) RecordPayment(ctx context.Context, userID uuid.UUID, dto *billingModels.RecordPaymentDto) (*billingModels.Payment, error) {
	return s.repo.RecordPayment(ctx, userID, dto)
}

func (s *billingService) ListPayments(ctx context.Context, userID uuid.UUID, page, limit int) (*billingModels.PaymentListResponse, error) {
	return s.repo.ListPayments(ctx, userID, page, limit)
}
