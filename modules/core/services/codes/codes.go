// Package codes draws QR codes and barcodes for every module (CORE-003): pure Go, PNG for a PDF, SVG for a
// browser that prints labels. A screen draws its own QR with the design system's component (D1).
package codes

import (
	"bytes"
	"errors"
	"fmt"
	"image/png"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/ean"
	"github.com/skip2/go-qrcode"
)

// The barcode symbologies this package draws.
const (
	EAN13   = "ean13"
	Code128 = "code128"
)

// maxData is more than any printable barcode or QR on a receipt ever holds.
const maxData = 2000

// ErrInvalidData is data the symbology cannot encode: a wrong EAN-13 check digit, letters in an EAN.
var ErrInvalidData = errors.New("codes.invalid-data")

// QR is a square PNG of size pixels, error correction M.
func QR(data string, size int) ([]byte, error) {
	code, err := newQR(data)
	if err != nil {
		return nil, err
	}
	return code.PNG(size)
}

// QRSVG is the same QR as an SVG of size user units.
func QRSVG(data string, size int) ([]byte, error) {
	code, err := newQR(data)
	if err != nil {
		return nil, err
	}
	bitmap := code.Bitmap()
	n := len(bitmap)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" shape-rendering="crispEdges"><rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, size, size, n, n, n, n)
	for y, row := range bitmap {
		for x, dark := range row {
			if dark {
				fmt.Fprintf(&b, "M%d %dh1v1h-1z", x, y)
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return []byte(b.String()), nil
}

func newQR(data string) (*qrcode.QRCode, error) {
	if data == "" || len(data) > maxData {
		return nil, ErrInvalidData
	}
	code, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return nil, ErrInvalidData
	}
	return code, nil
}

// Barcode is a PNG of w by h pixels.
func Barcode(symbology, data string, w, h int) ([]byte, error) {
	code, err := newBarcode(symbology, data)
	if err != nil {
		return nil, err
	}
	scaled, err := barcode.Scale(code, w, h)
	if err != nil {
		return nil, ErrInvalidData
	}
	var out bytes.Buffer
	if err := png.Encode(&out, scaled); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// BarcodeSVG is the same barcode as an SVG of w by h user units; each run of dark modules is one rect.
func BarcodeSVG(symbology, data string, w, h int) ([]byte, error) {
	code, err := newBarcode(symbology, data)
	if err != nil {
		return nil, err
	}
	modules := code.Bounds().Dx()
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d 1" preserveAspectRatio="none" shape-rendering="crispEdges"><rect width="%d" height="1" fill="#fff"/>`, w, h, modules, modules)
	for x := 0; x < modules; x++ {
		if !isDark(code, x) {
			continue
		}
		start := x
		for x+1 < modules && isDark(code, x+1) {
			x++
		}
		fmt.Fprintf(&b, `<rect x="%d" width="%d" height="1"/>`, start, x-start+1)
	}
	b.WriteString(`</svg>`)
	return []byte(b.String()), nil
}

func isDark(code barcode.Barcode, x int) bool {
	r, _, _, _ := code.At(x, 0).RGBA()
	return r == 0
}

func newBarcode(symbology, data string) (barcode.Barcode, error) {
	if data == "" || len(data) > maxData {
		return nil, ErrInvalidData
	}
	var (
		code barcode.Barcode
		err  error
	)
	switch symbology {
	case EAN13:
		if len(data) != 13 {
			return nil, ErrInvalidData
		}
		code, err = ean.Encode(data)
	case Code128:
		code, err = code128.Encode(data)
	default:
		return nil, ErrInvalidData
	}
	if err != nil {
		return nil, ErrInvalidData
	}
	return code, nil
}
