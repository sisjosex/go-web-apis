// Package api describes every JSON route as a huma operation mounted on the Gin engine (INFRA-021):
// the OpenAPI contract comes from the handler's input and output types, not from comments.
//
// Binding stays Gin's: a handler reads its body and query through Body[T] and Query[T], which call
// ShouldBindJSON and ShouldBindQuery, so validation, defaults and error bodies are what they were.
// huma routes, documents and writes the response.
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sort"
	"sync"

	"github.com/danielgtaylor/huma/v2"
)

// Title and Version head the contract.
const (
	Title   = "Taypi API"
	Version = "1"
)

// route is what the contract knows about one registered operation beyond OpenAPI: the Go handler it
// runs and the module that owns it.
type route struct {
	Method  string
	Path    string
	Handler string
	Module  string
}

type contract struct {
	mu     sync.Mutex
	config huma.Config
	routes map[string]route // "METHOD /gin/path" → route
	opIDs  map[string]int
	// requests are the request body types; their `binding` tags shape their schemas.
	requests map[reflect.Type]bool
	settled  bool
}

var current = newContract()

// Reset starts an empty contract. routes.SetupRoutes calls it before registering anything, so a
// process — or a test binary building a second engine — never carries routes from an earlier one.
func Reset() {
	c := newContract()
	current.mu.Lock()
	defer current.mu.Unlock()
	current.config, current.routes, current.opIDs, current.requests, current.settled = c.config, c.routes, c.opIDs, c.requests, false
}

func newContract() *contract {
	registry := huma.NewMapRegistry("#/components/schemas/", schemaName)
	aliasWireTypes(registry)
	return &contract{
		config: huma.Config{
			OpenAPI: &huma.OpenAPI{
				OpenAPI: "3.1.0",
				Info:    &huma.Info{Title: Title, Version: Version},
				Components: &huma.Components{
					Schemas: registry,
					SecuritySchemes: map[string]*huma.SecurityScheme{
						bearerScheme: {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
					},
				},
			},
			Formats:       map[string]huma.Format{"application/json": ginJSON, "json": ginJSON},
			DefaultFormat: "application/json",
		},
		routes:   map[string]route{},
		opIDs:    map[string]int{},
		requests: map[reflect.Type]bool{},
	}
}

// ginJSON writes what gin's c.JSON writes: json.Marshal (HTML escaped, no trailing newline).
var ginJSON = huma.Format{
	Marshal: func(w io.Writer, v any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	},
	Unmarshal: json.Unmarshal,
}

// OpenAPI is the contract of every operation registered since the last Reset, with the request
// schemas' required fields and enums taken from their `binding` tags.
func OpenAPI() *huma.OpenAPI {
	current.mu.Lock()
	defer current.mu.Unlock()
	if !current.settled {
		applyWireNullability(current.config.Components.Schemas)
		applyBindingTags(current.config.Components.Schemas, current.requests)
		current.settled = true
	}
	return current.config.OpenAPI
}

// JSON is the contract as OpenAPI 3.0, indented: what docs/openapi.json holds and /openapi.json
// serves. 3.0 rather than huma's 3.1 because the breaking-change check and the client generators
// read it.
func JSON() ([]byte, error) {
	down, err := OpenAPI().Downgrade()
	if err != nil {
		return nil, err
	}
	var v any
	if err := json.Unmarshal(down, &v); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// HandlerOf names the Go handler behind a Gin route ("METHOD", "/api/v1/users/:id"), or "" when the
// route was not registered through this package. The route-coverage recorder reads the module from it.
func HandlerOf(method, path string) string {
	current.mu.Lock()
	defer current.mu.Unlock()
	return current.routes[method+" "+path].Handler
}

// Route is one operation as the system map lists it.
type Route struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operation_id"`
	Module      string `json:"module"`
	Handler     string `json:"handler"`
}

// Routes lists every registered operation, sorted by path then method.
func Routes() []Route {
	spec := OpenAPI()
	current.mu.Lock()
	defer current.mu.Unlock()
	var out []Route
	for path, item := range spec.Paths {
		for method, op := range operations(item) {
			r := current.routes[method+" "+ginPath(path)]
			out = append(out, Route{Method: method, Path: path, OperationID: op.OperationID, Module: r.Module, Handler: r.Handler})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}

func operations(item *huma.PathItem) map[string]*huma.Operation {
	ops := map[string]*huma.Operation{}
	for method, op := range map[string]*huma.Operation{
		http.MethodGet: item.Get, http.MethodPost: item.Post, http.MethodPut: item.Put,
		http.MethodPatch: item.Patch, http.MethodDelete: item.Delete,
	} {
		if op != nil {
			ops[method] = op
		}
	}
	return ops
}
