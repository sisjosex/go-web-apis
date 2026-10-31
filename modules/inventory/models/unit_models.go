package models

// Unit is one of the units a business can use (INV-023 D2).
type Unit struct {
	ID            string  `json:"id"`
	Code          string  `json:"code"`
	NameEs        string  `json:"name_es"`
	NameEn        string  `json:"name_en"`
	Symbol        string  `json:"symbol"`
	SiatCode      *string `json:"siat_code"`
	AllowFraction bool    `json:"allow_fraction"`
}

// ListUnitsResponse is GET /inventory/units.
type ListUnitsResponse struct {
	Units []Unit `json:"units"`
}

// ProductUnitCode is one barcode of a presentation, for one combination.
type ProductUnitCode struct {
	SkuID string `json:"sku_id" binding:"required,uuid"`
	Code  string `json:"code" binding:"required,max=64"`
}

// ProductUnit is one presentation of a product: factor base units, its price and its codes.
type ProductUnit struct {
	ID                string            `json:"id"`
	UnitID            string            `json:"unit_id"`
	Code              string            `json:"code"`
	NameEs            string            `json:"name_es"`
	NameEn            string            `json:"name_en"`
	Symbol            string            `json:"symbol"`
	Factor            float64           `json:"factor"`
	IsBase            bool              `json:"is_base"`
	AllowFraction     bool              `json:"allow_fraction"`
	Sellable          bool              `json:"sellable"`
	PosVisible        bool              `json:"pos_visible"`
	IsSaleDefault     bool              `json:"is_sale_default"`
	IsPurchaseDefault bool              `json:"is_purchase_default"`
	Price             *float64          `json:"price"`
	Codes             []ProductUnitCode `json:"codes"`
}

// ProductUnitDto is one row of PUT /inventory/products/:id/units; with an id it updates that presentation.
type ProductUnitDto struct {
	ID                *string           `json:"id" binding:"omitempty,uuid"`
	UnitID            string            `json:"unit_id" binding:"required,uuid"`
	Factor            float64           `json:"factor" binding:"required,gt=0"`
	IsBase            bool              `json:"is_base"`
	AllowFraction     bool              `json:"allow_fraction"`
	Sellable          bool              `json:"sellable"`
	PosVisible        bool              `json:"pos_visible"`
	IsSaleDefault     bool              `json:"is_sale_default"`
	IsPurchaseDefault bool              `json:"is_purchase_default"`
	Price             *float64          `json:"price" binding:"omitempty,gte=0"`
	Codes             []ProductUnitCode `json:"codes" binding:"dive"`
}

// ReplaceProductUnitsDto is the product's whole list of presentations.
type ReplaceProductUnitsDto struct {
	Units []ProductUnitDto `json:"units" binding:"required,min=1,dive"`
}

// CodeLookupProduct is the product a code names.
type CodeLookupProduct struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// CodeLookupSku is the combination a code names.
type CodeLookupSku struct {
	ID  string `json:"id"`
	SKU string `json:"sku"`
}

// CodeLookupUnit is the presentation a code names.
type CodeLookupUnit struct {
	ID     string `json:"id"`
	NameEs string `json:"name_es"`
	NameEn string `json:"name_en"`
}

// CodeLookup is GET /inventory/codes/:code.
type CodeLookup struct {
	Product CodeLookupProduct `json:"product"`
	Sku     CodeLookupSku     `json:"sku"`
	Unit    CodeLookupUnit    `json:"unit"`
	Factor  float64           `json:"factor"`
	Price   float64           `json:"price"`
}
