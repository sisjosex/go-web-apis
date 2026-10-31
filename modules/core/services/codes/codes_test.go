package codes

import (
	"bytes"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQR_PNGOfTheAskedSize(t *testing.T) {
	data, err := QR("https://taypi24.com/pagar/abc", 240)
	require.NoError(t, err)

	img, err := png.Decode(bytes.NewReader(data))

	require.NoError(t, err)
	assert.Equal(t, 240, img.Bounds().Dx())
	assert.Equal(t, 240, img.Bounds().Dy())
}

func TestQRSVG_DrawsModules(t *testing.T) {
	data, err := QRSVG("hola", 120)

	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(string(data), "<svg"))
	assert.Contains(t, string(data), `width="120"`)
	assert.Contains(t, string(data), "h1v1h-1z")
}

func TestBarcode_ValidAndInvalid(t *testing.T) {
	for _, tc := range []struct {
		symbology, data string
		ok              bool
	}{
		{EAN13, "7501031311309", true},
		{EAN13, "7501031311308", false},
		{EAN13, "750103131130", false},
		{EAN13, "ABCDEFGHIJKLM", false},
		{Code128, "POL-BAS-M", true},
		{Code128, "", false},
		{"pdf417", "x", false},
	} {
		png, err := Barcode(tc.symbology, tc.data, 240, 80)
		svg, svgErr := BarcodeSVG(tc.symbology, tc.data, 240, 80)
		if tc.ok {
			assert.NoError(t, err, tc.data)
			assert.NotEmpty(t, png)
			assert.NoError(t, svgErr, tc.data)
			assert.Contains(t, string(svg), "<rect x=")
		} else {
			assert.ErrorIs(t, err, ErrInvalidData, tc.data)
			assert.ErrorIs(t, svgErr, ErrInvalidData, tc.data)
		}
	}
}
