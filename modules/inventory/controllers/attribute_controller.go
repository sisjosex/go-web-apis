package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	inventoryErrors "josex/web/modules/inventory/errors"
	"josex/web/modules/inventory/interfaces"
	"josex/web/modules/inventory/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

// facetsTTL keeps the filters' counts for a minute: they are read on every list view and only move on a write.
const (
	facetsTTL        = 60 * time.Second
	facetsMaxEntries = 1000
)

type facetsEntry struct {
	facets  []models.ProductFacet
	expires time.Time
}

// AttributeController serves attributes, what a category brings, the categories' order and the list's facets (INV-024).
type AttributeController struct {
	attributes interfaces.AttributeRepository
	facetsMu   sync.Mutex
	facets     map[string]facetsEntry
}

// NewAttributeController builds the controller.
func NewAttributeController(attributes interfaces.AttributeRepository) *AttributeController {
	return &AttributeController{attributes: attributes, facets: map[string]facetsEntry{}}
}

// attributeRefusal names the attribute a value failed on, which only the database knows.
func attributeRefusal(c *gin.Context, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Message == inventoryErrors.AttributeInvalid {
		return api.Fail(http.StatusUnprocessableEntity, coreErrors.BuildErrorDetail(c, pgErr.Message, pgErr.Detail))
	}
	return failUnit(c, err)
}

// refusedAttribute is a product write the attributes turned down; nil lets the write's own mapping go on.
func refusedAttribute(c *gin.Context, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Message, "inventory.attribute.") {
		return attributeRefusal(c, err)
	}
	return nil
}

// attrFilters reads attr[code]=a,b from a query string as {code: [a, b]}; none is nil.
func attrFilters(values url.Values) (*string, error) {
	filters := map[string][]string{}
	for key, raw := range values {
		if !strings.HasPrefix(key, "attr[") || !strings.HasSuffix(key, "]") {
			continue
		}
		code := strings.TrimSuffix(strings.TrimPrefix(key, "attr["), "]")
		for _, joined := range raw {
			for _, value := range strings.Split(joined, ",") {
				if value = strings.TrimSpace(value); value != "" {
					filters[code] = append(filters[code], value)
				}
			}
		}
	}
	if len(filters) == 0 {
		return nil, nil
	}
	body, err := json.Marshal(filters)
	if err != nil {
		return nil, err
	}
	out := string(body)
	return &out, nil
}

// ListAttributes is GET /attributes.
func (ctrl *AttributeController) ListAttributes(ctx context.Context, _ *api.None) (*api.Out[models.ListAttributesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	rows, err := ctrl.attributes.ListAttributes(c.Request.Context(), tenantID, nil)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.ListAttributesResponse{Attributes: rows})
}

// CreateAttributeInput is CreateAttribute's request.
type CreateAttributeInput struct {
	JSON api.Body[models.UpsertAttributeDto]
}

// CreateAttribute is POST /attributes.
func (ctrl *AttributeController) CreateAttribute(ctx context.Context, in *CreateAttributeInput) (*api.Out[models.Attribute], error) {
	c := api.Gin(ctx)
	var dto models.UpsertAttributeDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	saved, err := ctrl.upsert(c, nil, dto)
	if err != nil {
		return nil, err
	}
	return api.OK(saved)
}

// UpdateAttributeInput is UpdateAttribute's request.
type UpdateAttributeInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.UpsertAttributeDto]
}

// UpdateAttribute is PUT /attributes/{id}: the whole definition, options diffed by id.
func (ctrl *AttributeController) UpdateAttribute(ctx context.Context, in *UpdateAttributeInput) (*api.Out[models.Attribute], error) {
	c := api.Gin(ctx)
	var dto models.UpsertAttributeDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	saved, err := ctrl.upsert(c, &in.ID, dto)
	if err != nil {
		return nil, err
	}
	return api.OK(saved)
}

func (ctrl *AttributeController) upsert(c *gin.Context, id *string, dto models.UpsertAttributeDto) (models.Attribute, error) {
	tenantID, err := requestTenant(c)
	if err != nil {
		return models.Attribute{}, err
	}
	savedID, err := ctrl.attributes.UpsertAttribute(c.Request.Context(), tenantID, id, dto)
	if err != nil {
		return models.Attribute{}, attributeRefusal(c, err)
	}
	rows, err := ctrl.attributes.ListAttributes(c.Request.Context(), tenantID, &savedID)
	if err != nil || len(rows) == 0 {
		return models.Attribute{}, failUnit(c, err)
	}
	return rows[0], nil
}

// DeleteAttributeInput is DeleteAttribute's request.
type DeleteAttributeInput struct {
	ID string `path:"id"`
}

// DeleteAttributeResponse confirms it.
type DeleteAttributeResponse struct {
	Message string `json:"message"`
}

// DeleteAttribute is DELETE /attributes/{id}.
func (ctrl *AttributeController) DeleteAttribute(ctx context.Context, in *DeleteAttributeInput) (*api.Out[DeleteAttributeResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	if err := ctrl.attributes.DeleteAttribute(c.Request.Context(), tenantID, in.ID); err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(DeleteAttributeResponse{Message: "Attribute deleted"})
}

// CategoryAttributesInput is GetCategoryAttributes's request.
type CategoryAttributesInput struct {
	ID string `path:"id"`
}

// GetCategoryAttributes is GET /categories/{id}/attributes: its own and those inherited.
func (ctrl *AttributeController) GetCategoryAttributes(ctx context.Context, in *CategoryAttributesInput) (*api.Out[*models.CategoryAttributesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	out, err := ctrl.attributes.GetCategoryAttributes(c.Request.Context(), tenantID, in.ID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// SetCategoryAttributesInput is SetCategoryAttributes's request.
type SetCategoryAttributesInput struct {
	ID   string `path:"id"`
	JSON api.Body[models.SetCategoryAttributesDto]
}

// SetCategoryAttributes is PUT /categories/{id}/attributes.
func (ctrl *AttributeController) SetCategoryAttributes(ctx context.Context, in *SetCategoryAttributesInput) (*api.Out[*models.CategoryAttributesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.SetCategoryAttributesDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.attributes.SetCategoryAttributes(c.Request.Context(), tenantID, in.ID, dto.AttributeIDs); err != nil {
		return nil, failUnit(c, err)
	}
	out, err := ctrl.attributes.GetCategoryAttributes(c.Request.Context(), tenantID, in.ID)
	if err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(out)
}

// ReorderCategoriesInput is ReorderCategories's request.
type ReorderCategoriesInput struct {
	JSON api.Body[models.ReorderCategoriesDto]
}

// ReorderCategories is PATCH /categories/order: parent and place of each moved row, in one statement.
func (ctrl *AttributeController) ReorderCategories(ctx context.Context, in *ReorderCategoriesInput) (*api.Out[models.ReorderCategoriesResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var dto models.ReorderCategoriesDto
	if err := in.JSON.Into(ctx, &dto); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	if err := ctrl.attributes.ReorderCategories(c.Request.Context(), tenantID, dto.Items); err != nil {
		return nil, failUnit(c, err)
	}
	return api.OK(models.ReorderCategoriesResponse{Moved: len(dto.Items)})
}

// ProductFacetsInput is ProductFacets's request.
type ProductFacetsInput struct {
	Query api.Query[models.ProductFacetsQuery]
}

func facetsKey(tenantID uuid.UUID, query models.ProductFacetsQuery, attrs *string) string {
	parts := []string{tenantID.String(), query.Search}
	if query.CategoryID != nil {
		parts = append(parts, *query.CategoryID)
	}
	if attrs != nil {
		parts = append(parts, *attrs)
	}
	return strings.Join(parts, "|")
}

// ProductFacets is GET /products/facets, kept a minute per tenant and query.
func (ctrl *AttributeController) ProductFacets(ctx context.Context, in *ProductFacetsInput) (*api.Out[models.ProductFacetsResponse], error) {
	c := api.Gin(ctx)
	tenantID, err := requestTenant(c)
	if err != nil {
		return nil, err
	}
	var query models.ProductFacetsQuery
	if err := in.Query.Into(ctx, &query); err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	attrs, err := attrFilters(c.Request.URL.Query())
	if err != nil {
		return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorDetail(c, "validation.invalid", err.Error()))
	}
	key := facetsKey(tenantID, query, attrs)
	now := time.Now()
	ctrl.facetsMu.Lock()
	entry, hit := ctrl.facets[key]
	ctrl.facetsMu.Unlock()
	if hit && now.Before(entry.expires) {
		return api.OK(models.ProductFacetsResponse{Facets: entry.facets})
	}
	facets, err := ctrl.attributes.ProductFacets(c.Request.Context(), tenantID, query, attrs)
	if err != nil {
		return nil, failUnit(c, err)
	}
	ctrl.facetsMu.Lock()
	if len(ctrl.facets) >= facetsMaxEntries {
		ctrl.facets = map[string]facetsEntry{}
	}
	ctrl.facets[key] = facetsEntry{facets: facets, expires: now.Add(facetsTTL)}
	ctrl.facetsMu.Unlock()
	return api.OK(models.ProductFacetsResponse{Facets: facets})
}
