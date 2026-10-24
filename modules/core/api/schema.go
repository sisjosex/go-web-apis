package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"

	coreErrors "josex/web/modules/core/errors"
	coreModels "josex/web/modules/core/models"
)

// aliasWireTypes documents the types whose JSON is not their Go shape: a UUID is a string, not 16
// numbers; DateOnly a YYYY-MM-DD string; DateTime an RFC 3339 one; RawMessage any JSON.
func aliasWireTypes(r huma.Registry) {
	r.RegisterTypeAlias(reflect.TypeFor[uuid.UUID](), reflect.TypeFor[uuidWire]())
	r.RegisterTypeAlias(reflect.TypeFor[coreModels.DateOnly](), reflect.TypeFor[dateWire]())
	r.RegisterTypeAlias(reflect.TypeFor[coreModels.DateTime](), reflect.TypeFor[time.Time]())
	r.RegisterTypeAlias(reflect.TypeFor[json.RawMessage](), reflect.TypeFor[anyWire]())
}

type (
	uuidWire string
	dateWire string
	anyWire  struct{}
)

func (uuidWire) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeString, Format: "uuid"}
}

func (dateWire) Schema(huma.Registry) *huma.Schema {
	return &huma.Schema{Type: huma.TypeString, Format: "date"}
}

func (anyWire) Schema(huma.Registry) *huma.Schema { return &huma.Schema{} }

// wireTypes are the aliased types: huma resolves an alias from the pointed-to type, so a pointer to
// one loses the nullability any other pointer gets.
var wireTypes = map[reflect.Type]bool{
	reflect.TypeFor[uuid.UUID]():           true,
	reflect.TypeFor[coreModels.DateOnly](): true,
	reflect.TypeFor[coreModels.DateTime](): true,
}

// applyWireNullability gives a pointer to a wire type back its null — as huma does for any other
// pointer: unless the field is omitempty (never sent as null) or says nullable:"false".
func applyWireNullability(r huma.Registry) {
	for name, s := range r.Map() {
		t := r.TypeFromRef("#/components/schemas/" + name)
		if t == nil || t.Kind() != reflect.Struct {
			continue
		}
		for _, f := range fields(t) {
			if f.Type.Kind() != reflect.Pointer || !wireTypes[f.Type.Elem()] || f.Tag.Get("nullable") == "false" {
				continue
			}
			jsonName, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
			if jsonName == "" {
				jsonName = f.Name
			}
			if p := s.Properties[jsonName]; p != nil && p.Ref == "" && !strings.Contains(opts, "omitempty") {
				p.Nullable = true
			}
		}
	}
}

var rePkgModule = regexp.MustCompile(`/modules/([a-z_]+)(/|$)`)

// schemaName is "module.Type" (`tracking.Driver`, `core.ErrorResponse`), so two modules' types of the
// same name never collide and a client sees where a type lives. Types outside modules keep huma's name.
func schemaName(t reflect.Type, hint string) string {
	name := huma.DefaultSchemaNamer(t, hint)
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if m := rePkgModule.FindStringSubmatch(t.PkgPath()); m != nil && t.Name() != "" {
		return m[1] + "." + name
	}
	return name
}

// applyBindingTags makes a request body's schema say what its validator enforces: it requires
// exactly its `binding:"required"` fields (not every field without omitempty, huma's rule, which
// stays right for responses), a pointer field takes null, and `oneof`/`min`/`max` become enum and
// bounds. Structs nested in a request body are requests too.
func applyBindingTags(r huma.Registry, requests map[reflect.Type]bool) {
	inRequest := map[reflect.Type]bool{}
	for t := range requests {
		collectStructs(t, inRequest)
	}
	for name, s := range r.Map() {
		t := r.TypeFromRef("#/components/schemas/" + name)
		if t == nil || !inRequest[t] {
			continue
		}
		s.Required = bindFields(s, t)
	}
}

// bindFields applies each field's binding tag to its property in s and answers the required ones.
func bindFields(s *huma.Schema, t reflect.Type) []string {
	var required []string
	for _, f := range fields(t) {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" {
			name = f.Name
		}
		if name == "-" {
			continue
		}
		bind := f.Tag.Get("binding")
		if hasRule(bind, "required") {
			required = append(required, name)
		}
		if p := s.Properties[name]; p != nil {
			applyBinding(p, bind)
			// Go decodes null into a pointer: the request may send it
			if f.Type.Kind() == reflect.Pointer && p.Ref == "" {
				p.Nullable = true
			}
		}
	}
	return required
}

// collectStructs adds t and every struct type reachable through its fields.
func collectStructs(t reflect.Type, seen map[reflect.Type]bool) {
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array || t.Kind() == reflect.Map {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || seen[t] {
		return
	}
	seen[t] = true
	for _, f := range fields(t) {
		collectStructs(f.Type, seen)
	}
}

// fields are t's fields with embedded structs flattened, as encoding/json sees them.
func fields(t reflect.Type) []reflect.StructField {
	var out []reflect.StructField
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct && f.Tag.Get("json") == "" {
			out = append(out, fields(f.Type)...)
			continue
		}
		if f.IsExported() {
			out = append(out, f)
		}
	}
	return out
}

func hasRule(bind, rule string) bool {
	for _, r := range strings.Split(bind, ",") {
		if r == rule {
			return true
		}
	}
	return false
}

// applyBinding copies the validator rules a client can check before sending into the schema. A $ref
// schema is left alone: its rules belong to the referenced type.
func applyBinding(s *huma.Schema, bind string) {
	if s == nil || s.Ref != "" || bind == "" {
		return
	}
	for _, rule := range strings.Split(bind, ",") {
		key, val, _ := strings.Cut(rule, "=")
		switch key {
		case "oneof":
			s.Enum = nil
			for _, v := range strings.Fields(val) {
				s.Enum = append(s.Enum, v)
			}
		case "min", "gte":
			bound(s, val, true)
		case "max", "lte":
			bound(s, val, false)
		case "email", "email-valid":
			s.Format = "email"
		case "uuid", "uuid4":
			s.Format = "uuid"
		}
	}
}

func bound(s *huma.Schema, val string, lower bool) {
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return
	}
	switch s.Type {
	case huma.TypeString:
		i := int(n)
		if lower {
			s.MinLength = &i
		} else {
			s.MaxLength = &i
		}
	case huma.TypeArray:
		i := int(n)
		if lower {
			s.MinItems = &i
		} else {
			s.MaxItems = &i
		}
	case huma.TypeInteger, huma.TypeNumber:
		if lower {
			s.Minimum = &n
		} else {
			s.Maximum = &n
		}
	}
}

// Schema documents every error response as core's ErrorResponse.
func (e *Error) Schema(r huma.Registry) *huma.Schema {
	return r.Schema(reflect.TypeFor[coreErrors.ErrorResponse](), true, "ErrorResponse")
}

// huma's own refusals — an operation's error responses in the docs, and a request huma itself could
// not parse — use the same ErrorResponse body every handler writes.
func init() {
	// A list the API answers is a list: the handlers build them non-nil (an empty one is []), so
	// clients get T[] / List<T>, not T[] | null on every collection.
	huma.DefaultArrayNullable = false
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		return &Error{Status: status, Body: &coreErrors.ErrorResponse{Error: msg}}
	}
	huma.NewErrorWithContext = func(ctx huma.Context, status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
		}
		detail := make([]string, 0, len(errs))
		for _, err := range errs {
			detail = append(detail, err.Error())
		}
		body := &coreErrors.ErrorResponse{Error: msg}
		if len(detail) > 0 {
			body.Detail = detail
		}
		return &Error{Status: status, Body: body}
	}
}
