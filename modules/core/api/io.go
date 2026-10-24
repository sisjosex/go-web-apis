package api

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/gin-gonic/gin/binding"
)

// Body documents a JSON request body of type T, on an input field named JSON (a field named Body is
// huma's own, and huma would read the body). The handler binds it with Into, where Gin's
// ShouldBindJSON runs — so it chooses when, as the handler did before (a path check first, say).
type Body[T any] struct{ _ [0]T }

// Into binds the request body into dst (which may be pre-filled) and validates its `binding` tags.
func (Body[T]) Into(ctx context.Context, dst *T) error { return Gin(ctx).ShouldBindJSON(dst) }

// IntoCached binds a body a middleware already read with ShouldBindBodyWith (Gin keeps it in the
// context), so the handler binds the same bytes again.
func (Body[T]) IntoCached(ctx context.Context, dst *T) error {
	return Gin(ctx).ShouldBindBodyWith(dst, binding.JSON)
}

func (Body[T]) bodyType() reflect.Type { return reflect.TypeFor[T]() }

// Query documents the query parameters of T — its `form` tags — and binds them with Into.
type Query[T any] struct{ _ [0]T }

// Into binds the query string into dst with Gin's ShouldBindQuery.
func (Query[T]) Into(ctx context.Context, dst *T) error { return Gin(ctx).ShouldBindQuery(dst) }

func (Query[T]) queryType() reflect.Type { return reflect.TypeFor[T]() }

// Form documents a multipart/form-data body whose fields are T's (`json` names; a file is a string
// with `format:"binary"`). The handler reads it through Gin (FormFile, PostForm), as before.
type Form[T any] struct{ _ [0]T }

func (Form[T]) formType() reflect.Type { return reflect.TypeFor[T]() }

// None is the input of a route that reads nothing from the request.
type None struct{}

// ByID is the input of a route whose only parameter is `{id}`. The handler parses it, so a bad UUID
// answers what that handler always answered.
type ByID struct {
	ID string `path:"id"`
}

// Out is a JSON response body.
type Out[T any] struct {
	Body T
}

// OK wraps v as the response body; the status is the route's Doc.Status.
func OK[T any](v T) (*Out[T], error) { return &Out[T]{Body: v}, nil }

// Reply is a response whose success status the handler picks (200 or 202, say).
type Reply[T any] struct {
	Status int
	Body   T
}

// Respond answers v with status.
func Respond[T any](status int, v T) (*Reply[T], error) {
	return &Reply[T]{Status: status, Body: v}, nil
}

// Empty is a response with no body: the route's Doc.Status, 204 normally.
type Empty struct{}

// Done answers the route's status with no body.
func Done() (*Empty, error) { return nil, nil }

// Cached is a revalidated read: its ETag and Cache-Control, and 304 with no body when the client
// already holds this version.
type Cached[T any] struct {
	Status       int
	ETag         string `header:"ETag"`
	CacheControl string `header:"Cache-Control"`
	Body         T
}

// Revalidate answers v under etag, or 304 when ifNoneMatch already is that etag.
func Revalidate[T any](ifNoneMatch, etag, cacheControl string, v T) (*Cached[T], error) {
	status := http.StatusOK
	if ifNoneMatch == etag {
		status = http.StatusNotModified
	}
	return &Cached[T]{Status: status, ETag: etag, CacheControl: cacheControl, Body: v}, nil
}

// RawJSON is a body already encoded as T's JSON (a read the SP wrote whole): it goes out as is, and
// the contract documents T.
type RawJSON[T any] []byte

// MarshalJSON writes the encoded bytes.
func (r RawJSON[T]) MarshalJSON() ([]byte, error) { return r, nil }

// Schema documents the body as T.
func (RawJSON[T]) Schema(r huma.Registry) *huma.Schema {
	return r.Schema(reflect.TypeFor[T](), true, "")
}

// NoStore is a body the client must not cache.
type NoStore[T any] struct {
	CacheControl string `header:"Cache-Control"`
	Body         T
}

// Uncached answers v with Cache-Control: no-store.
func Uncached[T any](v T) (*NoStore[T], error) {
	return &NoStore[T]{CacheControl: "no-store", Body: v}, nil
}

// File is a download: raw bytes with their type and, for an attachment, its disposition.
type File struct {
	ContentType        string `header:"Content-Type"`
	ContentDisposition string `header:"Content-Disposition"`
	Body               []byte
}

// Error is a response other than the route's success: a status and the body to write, normally a
// core ErrorResponse.
type Error struct {
	Status int
	Body   any
}

// Fail is the error a handler returns to answer status with body.
func Fail(status int, body any) error { return &Error{Status: status, Body: body} }

func (e *Error) Error() string {
	b, _ := json.Marshal(e.Body)
	return http.StatusText(e.Status) + ": " + string(b)
}

// GetStatus makes Error a huma.StatusError, so huma writes it with its status.
func (e *Error) GetStatus() int { return e.Status }

// MarshalJSON writes the body alone.
func (e *Error) MarshalJSON() ([]byte, error) { return json.Marshal(e.Body) }

type bodyDoc interface{ bodyType() reflect.Type }
type formDoc interface{ formType() reflect.Type }
type queryDoc interface{ queryType() reflect.Type }

// documentInputs adds the request body and query parameters that Body and Query fields of the input
// type stand for; huma documents path parameters from their tags on its own.
func documentInputs(op *huma.Operation, in reflect.Type) {
	for i := 0; i < in.NumField(); i++ {
		f := in.Field(i)
		v := reflect.New(f.Type).Elem().Interface()
		if b, ok := v.(bodyDoc); ok {
			current.mu.Lock()
			current.requests[b.bodyType()] = true
			current.settled = false
			current.mu.Unlock()
			op.RequestBody = &huma.RequestBody{
				Required: true,
				Content:  map[string]*huma.MediaType{"application/json": {Schema: schemaOf(b.bodyType())}},
			}
		}
		if fd, ok := v.(formDoc); ok {
			op.RequestBody = &huma.RequestBody{
				Required: true,
				Content:  map[string]*huma.MediaType{"multipart/form-data": {Schema: schemaOf(fd.formType())}},
			}
		}
		if q, ok := v.(queryDoc); ok {
			op.Parameters = append(op.Parameters, queryParams(q.queryType())...)
		}
	}
}

// schemaOf is t's schema in the contract's registry: a $ref for a named struct.
func schemaOf(t reflect.Type) *huma.Schema {
	return current.config.Components.Schemas.Schema(t, true, t.Name())
}

// queryParams lists T's `form` fields as query parameters; embedded structs are walked.
func queryParams(t reflect.Type) []*huma.Param {
	var out []*huma.Param
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			out = append(out, queryParams(f.Type)...)
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("form"), ",")
		if name == "" || name == "-" {
			continue
		}
		ft := f.Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		s := schemaOf(ft)
		if def, ok := strings.CutPrefix(opts, "default="); ok {
			s = withDefault(s, def)
		}
		bind := f.Tag.Get("binding")
		applyBinding(s, bind)
		out = append(out, &huma.Param{
			Name:        name,
			In:          "query",
			Required:    hasRule(bind, "required"),
			Schema:      s,
			Description: f.Tag.Get("doc"),
		})
	}
	return out
}

func withDefault(s *huma.Schema, def string) *huma.Schema {
	c := *s
	c.Default = def
	return &c
}
