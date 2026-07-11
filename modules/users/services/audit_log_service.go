package services

import (
	"encoding/json"

	"github.com/google/uuid"

	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
)

type userAuditService struct {
	userAuditRepository userInterfaces.UserAuditRepository
}

func NewUserAuditService(repo userInterfaces.UserAuditRepository) userInterfaces.UserAuditService {
	return &userAuditService{userAuditRepository: repo}
}

func (s *userAuditService) Record(tenantID uuid.UUID, action string, targetUserID, performedBy *uuid.UUID, metadata *json.RawMessage, source string) error {
	return s.userAuditRepository.Record(tenantID, action, targetUserID, performedBy, metadata, source)
}

func (s *userAuditService) List(query userModels.UserAuditListQuery) (*userModels.UserAuditListResponse, error) {
	return s.userAuditRepository.List(query)
}
