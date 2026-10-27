//go:build integration
// +build integration

package tracking_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/xuri/excelize/v2"

	"josex/web/modules/core/testhelpers"
)

// routesWorkbook is a carrier's routes sheet as Excel saves it, with its own Spanish headings.
func routesWorkbook(t *testing.T, rows [][]any) []byte {
	t.Helper()
	book := excelize.NewFile()
	defer func() { _ = book.Close() }()
	sheet := book.GetSheetName(0)
	_ = book.SetSheetRow(sheet, "A1", &[]any{"Nombre", "Código", "Sentido", "Cliente", "Empresa", "Placa", "Conductor", "Hora", "Días"})
	for i, row := range rows {
		cellName, _ := excelize.CoordinatesToCellName(1, i+2)
		_ = book.SetSheetRow(sheet, cellName, &row)
	}
	buf, err := book.WriteToBuffer()
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func importRoutesWorkbook(t *testing.T, helper *testhelpers.ApiTestHelper, path string, book []byte) importResult {
	t.Helper()
	files := []testhelpers.MultipartFile{{Field: "file", Filename: "rutas.xlsx", Content: book}}
	w := helper.DoMultipartRequest("POST", path, map[string]string{"resource": "tracking_routes"}, files, map[string]string{})
	if w.Code != http.StatusOK {
		t.Fatalf("import routes returned %d: %s", w.Code, w.Body.String())
	}
	var response importResult
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode import response: %v — %s", err, w.Body.String())
	}
	return response
}

// TestImportRoutes - TRACK-075: an ingreso and the retorno of the same name become a pair, each with
// its schedule, code and driver; a row naming an unknown plate is reported and not created.
func TestImportRoutes(t *testing.T) {
	helper := SetupTrackingTest(t)
	defer helper.Close()
	tag := strings.ToUpper(uuid.NewString()[:5])
	org := "Import School " + tag
	w := helper.DoRequest("POST", "/tracking/organizations", map[string]interface{}{
		"name": org, "kind": "school", "timezone": "UTC", "latitude": -17.3935, "longitude": -66.1570,
	}, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create organization: %d %s", w.Code, w.Body.String())
	}
	orgID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	driverBody := ValidDriverBody()
	driverBody["last_name"] = "Imp" + tag
	w = helper.DoRequest("POST", "/tracking/drivers", driverBody, map[string]string{})
	if w.Code != http.StatusCreated {
		t.Fatalf("create driver: %d %s", w.Code, w.Body.String())
	}
	driverID := ExtractID(t, ParseResponse(t, w.Body.Bytes()))
	company := CompanyName(t, helper, MainCompanyID)
	name := "Ruta Norte " + tag
	book := routesWorkbook(t, [][]any{
		{name, "IN-" + tag, "Ingreso", org, company, "TST-BUS-001", "Ana Imp" + tag, "07:30", "lun-vie"},
		{name, "RE-" + tag, "Retorno", org, company, "", "", "13:15", "lun-vie"},
		{"Ruta Sur " + tag, "", "Ingreso", org, company, "NOPE-" + tag, "", "08:00", ""},
	})

	dry := importRoutesWorkbook(t, helper, "/import/validate", book)
	assert.Equal(t, []string{"valid", "valid", "invalid"}, rowStatuses(dry, 3))

	done := importRoutesWorkbook(t, helper, "/import", book)
	assert.Equal(t, []string{"created", "created", "failed"}, rowStatuses(done, 3))
	assert.Contains(t, done.rows(3)[2].Errors, "tracking.import.plate-unknown|NOPE-"+tag)

	w = helper.DoRequest("GET", "/tracking/routes?organization_id="+orgID, nil, map[string]string{})
	assert.Equal(t, http.StatusOK, w.Code)
	var list struct {
		Routes []struct {
			ID              string  `json:"id"`
			RouteCode       *string `json:"route_code"`
			Direction       string  `json:"direction"`
			PairedRouteID   *string `json:"paired_route_id"`
			DefaultDriverID *string `json:"default_driver_id"`
		} `json:"routes"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &list))
	if !assert.Len(t, list.Routes, 2) {
		return
	}
	byDirection := map[string]int{}
	for i, route := range list.Routes {
		byDirection[route.Direction] = i
		assert.NotNil(t, route.PairedRouteID, "the ingreso and its retorno are a pair")
		assert.Len(t, ListRouteSchedules(t, helper, route.ID), 1)
	}
	outbound := list.Routes[byDirection["outbound"]]
	inbound := list.Routes[byDirection["inbound"]]
	assert.Equal(t, "IN-"+tag, *outbound.RouteCode)
	assert.Equal(t, "RE-"+tag, *inbound.RouteCode)
	if assert.NotNil(t, outbound.DefaultDriverID) {
		assert.Equal(t, driverID, *outbound.DefaultDriverID)
	}

	again := importRoutesWorkbook(t, helper, "/import/validate", book)
	assert.Equal(t, "duplicate", rowStatuses(again, 3)[0], "a known code is an existing route")
}
