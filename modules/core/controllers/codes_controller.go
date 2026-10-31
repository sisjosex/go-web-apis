package controllers

import (
	"context"
	"errors"
	"net/http"

	"josex/web/modules/core/api"
	coreErrors "josex/web/modules/core/errors"
	"josex/web/modules/core/services/codes"
)

// A barcode's bytes depend only on its URL, so a browser keeps it for a year (CORE-003).
const barcodeCache = "public, max-age=31536000, immutable"

const (
	barcodeDefaultW = 240
	barcodeDefaultH = 80
	barcodeMaxSide  = 2000
)

// CodesController draws what a browser cannot: the barcodes it prints on labels. A QR on screen is the
// design system's own component (CORE-003 D1).
type CodesController struct{}

// NewCodesController builds the controller.
func NewCodesController() *CodesController { return &CodesController{} }

// BarcodeInput is Barcode's request.
type BarcodeInput struct {
	Symbology string `path:"symbology"`
	Data      string `query:"data"`
	W         int    `query:"w"`
	H         int    `query:"h"`
}

// BarcodeImage is an SVG the browser may keep.
type BarcodeImage struct {
	ContentType  string `header:"Content-Type"`
	CacheControl string `header:"Cache-Control"`
	Body         []byte
}

// Barcode is GET /codes/barcode/{symbology}: an SVG of the barcode.
func (ctrl *CodesController) Barcode(ctx context.Context, in *BarcodeInput) (*BarcodeImage, error) {
	c := api.Gin(ctx)
	w, h := barcodeSide(in.W, barcodeDefaultW), barcodeSide(in.H, barcodeDefaultH)
	image, err := codes.BarcodeSVG(in.Symbology, in.Data, w, h)
	if err != nil {
		if errors.Is(err, codes.ErrInvalidData) {
			return nil, api.Fail(http.StatusBadRequest, coreErrors.BuildErrorSingle(c, codes.ErrInvalidData.Error()))
		}
		return nil, api.Fail(http.StatusInternalServerError, coreErrors.BuildError(c, err))
	}
	return &BarcodeImage{ContentType: "image/svg+xml", CacheControl: barcodeCache, Body: image}, nil
}

func barcodeSide(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return min(value, barcodeMaxSide)
}
