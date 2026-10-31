package models

// AttributeOption is one value an attribute offers; swatch for the colour type.
type AttributeOption struct {
	ID     string  `json:"id"`
	Value  string  `json:"value"`
	Swatch *string `json:"swatch"`
	Sort   int     `json:"sort"`
}

// Attribute is a shared characteristic (INV-024 D1), with where it is used.
type Attribute struct {
	ID            string            `json:"id"`
	Code          string            `json:"code"`
	Name          string            `json:"name"`
	Type          string            `json:"type"`
	Unit          *string           `json:"unit"`
	Filterable    bool              `json:"filterable"`
	Axis          bool              `json:"axis"`
	Options       []AttributeOption `json:"options"`
	ProductCount  int64             `json:"product_count"`
	CategoryNames []string          `json:"category_names"`
}

// ListAttributesResponse is GET /inventory/attributes.
type ListAttributesResponse struct {
	Attributes []Attribute `json:"attributes"`
}

// AttributeOptionDto is one option of a write; with an id it updates that option.
type AttributeOptionDto struct {
	ID     *string `json:"id" binding:"omitempty,uuid"`
	Value  string  `json:"value" binding:"required,max=80"`
	Swatch *string `json:"swatch" binding:"omitempty,max=20"`
	Sort   *int    `json:"sort"`
}

// UpsertAttributeDto is POST and PUT /inventory/attributes: the whole definition.
type UpsertAttributeDto struct {
	Name       string               `json:"name" binding:"required,max=80"`
	Type       string               `json:"type" binding:"required,oneof=list color measure number text"`
	Unit       *string              `json:"unit" binding:"omitempty,max=12"`
	Filterable *bool                `json:"filterable"`
	Axis       *bool                `json:"axis"`
	Options    []AttributeOptionDto `json:"options" binding:"dive"`
}

// CategoryAttribute is an attribute as a category brings it.
type CategoryAttribute struct {
	ID      string            `json:"id"`
	Code    string            `json:"code"`
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Unit    *string           `json:"unit"`
	Axis    bool              `json:"axis"`
	Options []AttributeOption `json:"options"`
}

// CategoryAttributesResponse is GET and PUT /inventory/categories/:id/attributes.
type CategoryAttributesResponse struct {
	Own       []CategoryAttribute `json:"own"`
	Inherited []CategoryAttribute `json:"inherited"`
}

// SetCategoryAttributesDto names the category's own attributes.
type SetCategoryAttributesDto struct {
	AttributeIDs []string `json:"attribute_ids" binding:"required,dive,uuid"`
}

// CategoryOrderItem is where one category lands.
type CategoryOrderItem struct {
	ID           string  `json:"id" binding:"required,uuid"`
	ParentID     *string `json:"parent_id" binding:"omitempty,uuid"`
	DisplayOrder int     `json:"display_order"`
}

// ReorderCategoriesDto is PATCH /inventory/categories/order.
type ReorderCategoriesDto struct {
	Items []CategoryOrderItem `json:"items" binding:"required,min=1,dive"`
}

// ReorderCategoriesResponse says how many moved.
type ReorderCategoriesResponse struct {
	Moved int `json:"moved"`
}

// FacetOption is one value of a facet and how many products have it.
type FacetOption struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// ProductFacet is what the list's filters offer for one attribute.
type ProductFacet struct {
	Code    string        `json:"code"`
	Name    string        `json:"name"`
	Options []FacetOption `json:"options"`
}

// ProductFacetsResponse is GET /inventory/products/facets.
type ProductFacetsResponse struct {
	Facets []ProductFacet `json:"facets"`
}

// ProductFacetsQuery is GET /inventory/products/facets's filters; attr[code] travels beside them.
type ProductFacetsQuery struct {
	CategoryID *string `form:"category_id" binding:"omitempty,uuid"`
	Search     string  `form:"search"`
}
