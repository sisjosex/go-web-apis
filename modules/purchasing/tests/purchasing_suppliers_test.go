//go:build integration
// +build integration

package purchasing_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"josex/web/modules/purchasing/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PURCH-001: edit, deactivate and reactivate, search and status.

func TestSuppliers_DeactivateAndReactivate(t *testing.T) {
	helper := setupPurchasing(t)
	name := "Distribuidora " + stamp()
	w := helper.DoRequest("POST", "/purchasing/suppliers", map[string]interface{}{"name": name, "tax_id": "NIT" + stamp()}, nil)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created models.Supplier
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, name, created.Name, "create answers the whole supplier")
	assert.Equal(t, 30, created.PaymentTerms)
	id := created.ID.String()

	list := func(query string) models.Supplier {
		t.Helper()
		w := helper.DoRequest("GET", "/purchasing/suppliers?"+query, nil, nil)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		var page struct {
			Suppliers  []models.Supplier `json:"suppliers"`
			TotalCount int               `json:"total_count"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
		for _, row := range page.Suppliers {
			if row.ID == created.ID {
				return row
			}
		}
		return models.Supplier{}
	}
	assert.Equal(t, created.ID, list("search="+*created.TaxID).ID, "found by its tax id")

	w = helper.DoRequest("PATCH", "/purchasing/suppliers/"+id, map[string]interface{}{"phone": "70000000", "legal_name": "Andina SRL"}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var updated models.Supplier
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &updated))
	assert.Equal(t, "Andina SRL", *updated.LegalName)
	assert.Equal(t, name, updated.Name, "what is not sent stays")

	w = helper.DoRequest("DELETE", "/purchasing/suppliers/"+id, nil, nil)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	assert.NotEqual(t, created.ID, list("page_size=100").ID, "absent from the default list")
	assert.Equal(t, created.ID, list("status=inactive&page_size=100").ID)
	assert.Equal(t, created.ID, list("status=all&page_size=100").ID)

	w = helper.DoRequest("PATCH", "/purchasing/suppliers/"+id, map[string]interface{}{"is_active": true}, nil)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, created.ID, list("page_size=100").ID, "back in the default list")

	w = helper.DoRequest("GET", "/purchasing/suppliers?status=otro", nil, nil)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	w = helper.DoRequest("PATCH", "/purchasing/suppliers/00000000-0000-0000-0000-000000000000", map[string]interface{}{"phone": "1"}, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	w = helper.DoRequest("DELETE", "/purchasing/suppliers/00000000-0000-0000-0000-000000000000", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
}
