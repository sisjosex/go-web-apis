// Package contract writes and checks the API contract (INFRA-021): docs/openapi.json is the OpenAPI
// document of every JSON route, built from the handlers' types by mounting the routes the way the
// test suite does. `make contract` writes it; the gate's contract stage fails when it is stale.
package contract

import (
	"bytes"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	coreAPI "josex/web/modules/core/api"
	"josex/web/modules/core/testhelpers"
)

var write = flag.Bool("write", false, "rewrite docs/openapi.json instead of comparing")

func init() {
	testhelpers.InitTestEnvironment()
}

// TestContract mounts every route and compares the contract with the committed one.
func TestContract(t *testing.T) {
	helper := testhelpers.SetupApiTest(t)
	defer helper.Close()

	// Every JSON route is an operation of the contract; these are the routes that are not JSON.
	notJSON := map[string]bool{
		"GET /":                   true, // the root ping
		"GET /openapi.json":       true,
		"GET /docs":               true,
		"GET /api/v1/tracking/ws": true, // the WebSocket
	}
	var undocumented []string
	for _, r := range helper.Routes() {
		key := r.Method + " " + r.Path
		if notJSON[key] || strings.HasPrefix(r.Path, "/admin/jobs") || r.Method == http.MethodHead || r.Method == http.MethodOptions {
			continue
		}
		if coreAPI.HandlerOf(r.Method, r.Path) == "" {
			undocumented = append(undocumented, key+" ("+r.Handler+")")
		}
	}
	if len(undocumented) > 0 {
		t.Fatalf("routes registered with gin instead of api.Get/Post/…, so missing from the contract:\n  %s", strings.Join(undocumented, "\n  "))
	}

	spec, err := coreAPI.JSON()
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join("..", "..", "docs", "openapi.json")
	if *write {
		if err := os.WriteFile(path, spec, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v — run make contract", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")), spec) {
		t.Fatal("docs/openapi.json is stale: the routes' types changed — run make contract and commit it")
	}
}
