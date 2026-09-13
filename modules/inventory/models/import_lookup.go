package models

// ImportSkuResolution is what sp_import_resolve_sku_by_axes knows about the
// product a CSV cell names: nil ProductID when the code is unknown, the number
// of inventory axes it has, the combination whose option set equals the row's
// exactly, and its default bucket.
type ImportSkuResolution struct {
	ProductID    *string
	AxisCount    int
	MatchedSkuID *string
	MatchedSku   *string
	DefaultSkuID *string
	DefaultSku   *string
}

// ImportSkuExistence says whether a code is already taken by a product or by a
// combination of this tenant.
type ImportSkuExistence struct {
	ProductSkuTaken bool
	VariantSkuTaken bool
}
