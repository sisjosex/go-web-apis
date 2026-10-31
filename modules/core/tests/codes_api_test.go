//go:build integration
// +build integration

package core_test

import (
	"net/http"
	"testing"

	"josex/web/modules/core/testhelpers"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func codesHelper(t *testing.T) *testhelpers.ApiTestHelper {
	t.Helper()
	helper := testhelpers.SetupApiTest(t)
	_, err := helper.LoginAsSuperAdmin()
	require.NoError(t, err)
	helper.SetTenantSlug("test-company")
	return helper
}

// TestBarcode_SVGCachedForAYear - a valid code answers an SVG the browser keeps.
func TestBarcode_SVGCachedForAYear(t *testing.T) {
	helper := codesHelper(t)
	defer helper.Close()

	w := helper.DoRequest("GET", "/codes/barcode/ean13?data=7501031311309&w=300&h=90", nil, nil)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "image/svg+xml", w.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", w.Header().Get("Cache-Control"))
	assert.Contains(t, w.Body.String(), `width="300"`)
}

// TestBarcode_InvalidDataIs400 - a wrong check digit or an unknown symbology names the problem.
func TestBarcode_InvalidDataIs400(t *testing.T) {
	helper := codesHelper(t)
	defer helper.Close()

	for _, path := range []string{"/codes/barcode/ean13?data=7501031311308", "/codes/barcode/pdf417?data=1"} {
		w := helper.DoRequest("GET", path, nil, nil)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Contains(t, w.Body.String(), "codes.invalid-data")
	}
}
