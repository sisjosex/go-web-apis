package interfaces

import (
	"encoding/json"

	"github.com/google/uuid"

	userModels "josex/web/modules/users/models"
)

type UserAuditRepository interface {
	Record(tenantID uuid.UUID, action string, targetUserID, performedBy *uuid.UUID, metadata *json.RawMessage, source string) error
	List(query userModels.UserAuditListQuery) (*userModels.UserAuditListResponse, error)
}
