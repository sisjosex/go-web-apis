package controllers

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
)

type UserAuditController struct {
	userAuditService userInterfaces.UserAuditService
}

// ListAuditInput is GET /users/audit's query. A page below 1 is 1; a limit outside 1-100 is 20.
type ListAuditInput struct {
	Page   int    `query:"page" default:"1" doc:"Page number"`
	Limit  int    `query:"limit" default:"20" doc:"Items per page, at most 100"`
	Search string `query:"search" doc:"Search by target or performer email"`
	Action string `query:"action" doc:"Filter by action (e.g. user.created)"`
	From   string `query:"from" doc:"From date (RFC3339 or YYYY-MM-DD)"`
	To     string `query:"to" doc:"To date (RFC3339 or YYYY-MM-DD)"`
}

func NewUserAuditController(userAuditService userInterfaces.UserAuditService) *UserAuditController {
	return &UserAuditController{userAuditService: userAuditService}
}

func (ctrl *UserAuditController) ListAudit(ctx context.Context, in *ListAuditInput) (*api.Out[*userModels.UserAuditListResponse], error) {
	c := api.Gin(ctx)
	page, limit := in.Page, in.Limit

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	query := userModels.UserAuditListQuery{
		Page:   page,
		Limit:  limit,
		Search: in.Search,
		Action: in.Action,
	}

	if in.From != "" {
		query.From = &in.From
	}
	if in.To != "" {
		query.To = &in.To
	}

	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			query.TenantID = tid
		}
	}

	response, err := ctrl.userAuditService.List(query)
	if err != nil {
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, usersErrors.UserAuditListFailed))
	}

	return api.OK(response)
}
