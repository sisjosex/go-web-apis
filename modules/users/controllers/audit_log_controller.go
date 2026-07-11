package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	coreErrors "josex/web/modules/core/errors"
	usersErrors "josex/web/modules/users/errors"
	userInterfaces "josex/web/modules/users/interfaces"
	userModels "josex/web/modules/users/models"
)

type UserAuditController struct {
	userAuditService userInterfaces.UserAuditService
}

func NewUserAuditController(userAuditService userInterfaces.UserAuditService) *UserAuditController {
	return &UserAuditController{userAuditService: userAuditService}
}

// ListAudit godoc
// @Summary      List user audit log entries
// @Description  Returns paginated audit log entries for user management actions scoped to the current tenant
// @Tags         Users
// @Accept       json
// @Produce      json
// @Param        Authorization  header  string  true   "Bearer Token"
// @Param        page           query   int     false  "Page number (default: 1)"
// @Param        limit          query   int     false  "Items per page (default: 20, max: 100)"
// @Param        search         query   string  false  "Search by target or performer email"
// @Param        action         query   string  false  "Filter by action (e.g. user.created)"
// @Param        from           query   string  false  "Filter from date (RFC3339 or YYYY-MM-DD)"
// @Param        to             query   string  false  "Filter to date (RFC3339 or YYYY-MM-DD)"
// @Success      200  {object}  userModels.UserAuditListResponse
// @Failure      401  {object}  coreErrors.ErrorResponse
// @Failure      500  {object}  coreErrors.ErrorResponse
// @Router       /users/audit [get]
// @Security     ApiKeyAuth
func (ctrl *UserAuditController) ListAudit(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	query := userModels.UserAuditListQuery{
		Page:   page,
		Limit:  limit,
		Search: c.Query("search"),
		Action: c.Query("action"),
	}

	if from := c.Query("from"); from != "" {
		query.From = &from
	}
	if to := c.Query("to"); to != "" {
		query.To = &to
	}

	if tenantIDStr, ok := c.Get("tenant_id"); ok {
		if tid, err := uuid.Parse(tenantIDStr.(string)); err == nil {
			query.TenantID = tid
		}
	}

	response, err := ctrl.userAuditService.List(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, coreErrors.BuildErrorSingle(c, usersErrors.UserAuditListFailed))
		return
	}

	c.JSON(http.StatusOK, response)
}
