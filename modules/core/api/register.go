package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humagin"
	"github.com/gin-gonic/gin"
)

const bearerScheme = "bearer"

// Doc is what a route says about itself beyond its types.
type Doc struct {
	Summary     string
	Description string
	// Status is the success status; 200 when zero.
	Status int
	// Errors are the error statuses the handler answers, each with core's ErrorResponse body.
	Errors []int
	// Tags group the operation in the docs; the module's name when empty.
	Tags       []string
	Deprecated bool
}

// Handler is a huma operation: the parsed input in, the response body or an error out.
type Handler[I, O any] func(context.Context, *I) (*O, error)

// Get registers a GET operation on the group, behind the group's middleware and then mw.
func Get[I, O any](g *gin.RouterGroup, path string, h Handler[I, O], d Doc, mw ...gin.HandlerFunc) {
	register(g, http.MethodGet, path, h, d, mw)
}

// Post registers a POST operation.
func Post[I, O any](g *gin.RouterGroup, path string, h Handler[I, O], d Doc, mw ...gin.HandlerFunc) {
	register(g, http.MethodPost, path, h, d, mw)
}

// Put registers a PUT operation.
func Put[I, O any](g *gin.RouterGroup, path string, h Handler[I, O], d Doc, mw ...gin.HandlerFunc) {
	register(g, http.MethodPut, path, h, d, mw)
}

// Patch registers a PATCH operation.
func Patch[I, O any](g *gin.RouterGroup, path string, h Handler[I, O], d Doc, mw ...gin.HandlerFunc) {
	register(g, http.MethodPatch, path, h, d, mw)
}

// Delete registers a DELETE operation.
func Delete[I, O any](g *gin.RouterGroup, path string, h Handler[I, O], d Doc, mw ...gin.HandlerFunc) {
	register(g, http.MethodDelete, path, h, d, mw)
}

var reGinParam = regexp.MustCompile(`:([A-Za-z0-9_]+)`)

// register mounts h at the group's base path + path. path uses huma's `{param}` form; a Gin `:param`
// is accepted too.
func register[I, O any](g *gin.RouterGroup, method, path string, h Handler[I, O], d Doc, mw []gin.HandlerFunc) {
	full := reGinParam.ReplaceAllString(joinPath(g.BasePath(), path), "{$1}")
	handler, module := handlerName(h)

	op := huma.Operation{
		OperationID:      operationID(module, handler),
		Method:           method,
		Path:             full,
		Summary:          d.Summary,
		Description:      d.Description,
		Tags:             d.Tags,
		DefaultStatus:    d.Status,
		Errors:           d.Errors,
		Deprecated:       d.Deprecated,
		SkipValidateBody: true,
		MaxBodyBytes:     -1, // Gin reads the body: no huma limit or deadline on top of it.
		BodyReadTimeout:  -1,
	}
	// the Go handler, for the system map (tools/routemap) and for whoever reads the contract
	op.Extensions = map[string]any{"x-handler": handler}
	if len(op.Tags) == 0 && module != "" {
		op.Tags = []string{module}
	}
	if authenticated(g.Handlers, mw) {
		op.Security = []map[string][]string{{bearerScheme: {}}}
	}

	current.mu.Lock()
	config := current.config
	current.routes[method+" "+ginPath(full)] = route{Method: method, Path: full, Handler: handler, Module: module}
	current.settled = false
	current.mu.Unlock()

	// One API per route is cheap (no routes of its own) and carries the route's own middleware; every
	// one writes into the same OpenAPI document.
	a := huma.NewAPI(config, &groupAdapter{group: g, mw: mw})
	huma.Register(a, op, h)
	// After Register: an operation that declares a request body makes huma read it, and the body is
	// Gin's to bind.
	registered := operations(a.OpenAPI().Paths[full])[method]
	documentInputs(registered, reflect.TypeFor[I]())
	documentPathParams(registered, full)
	pruneResponses(registered, d.Errors)
}

// groupAdapter mounts a huma operation on a Gin group, so the group's middleware chain runs first and
// the route's own after it.
type groupAdapter struct {
	group *gin.RouterGroup
	mw    []gin.HandlerFunc
}

func (a *groupAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not served through huma", http.StatusNotFound)
}

func (a *groupAdapter) Handle(op *huma.Operation, handler func(huma.Context)) {
	rel := strings.TrimPrefix(ginPath(op.Path), a.group.BasePath())
	chain := append(append([]gin.HandlerFunc{}, a.mw...), func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), ginKey{}, c))
		handler(humagin.NewContext(op, c))
	})
	a.group.Handle(op.Method, rel, chain...)
}

type ginKey struct{}

// Gin is the request's Gin context, for what the middleware put there (tenant, user, language) and
// for the error builders that translate from it.
func Gin(ctx context.Context) *gin.Context {
	c, _ := ctx.Value(ginKey{}).(*gin.Context)
	return c
}

func joinPath(base, path string) string {
	if path == "" || path == "/" {
		if base == "" {
			return "/"
		}
		return base
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
}

var reHumaParam = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

// ginPath turns `/users/{id}` into `/users/:id`.
func ginPath(path string) string { return reHumaParam.ReplaceAllString(path, ":$1") }

var reModule = regexp.MustCompile(`/modules/([a-z_]+)/`)

// handlerName is the handler's Go name ("josex/web/modules/users/controllers.(*UserController).Create")
// and the module its package belongs to.
func handlerName(h any) (string, string) {
	name := strings.TrimSuffix(runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name(), "-fm")
	module := ""
	if m := reModule.FindStringSubmatch(name); m != nil {
		module = m[1]
	}
	return name, module
}

// operationID is "module.Method"; a handler mounted twice gets a numeric suffix on the second.
func operationID(module, handler string) string {
	method := handler[strings.LastIndex(handler, ".")+1:]
	id := method
	if module != "" {
		id = module + "." + method
	}
	current.mu.Lock()
	defer current.mu.Unlock()
	current.opIDs[id]++
	if n := current.opIDs[id]; n > 1 {
		return fmt.Sprintf("%s%d", id, n)
	}
	return id
}

// pruneResponses drops the error responses huma adds on its own (422, 500): the contract lists the
// success and the statuses the route declares. A request huma cannot parse answers 400.
func pruneResponses(op *huma.Operation, errs []int) {
	keep := map[string]bool{}
	for _, status := range errs {
		keep[strconv.Itoa(status)] = true
	}
	for status := range op.Responses {
		if status[0] != '2' && !keep[status] {
			delete(op.Responses, status)
		}
	}
}

// authenticated tells whether the route's chain — the group's middleware and its own — runs the
// bearer-token check, so the contract's security is the chain's, not a claim beside it.
func authenticated(chains ...[]gin.HandlerFunc) bool {
	for _, chain := range chains {
		for _, h := range chain {
			if strings.Contains(runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name(), "/modules/auth/middleware.AuthMiddleware") {
				return true
			}
		}
	}
	return false
}

// documentPathParams declares the path's parameters the input does not — a group's own
// (`/tenants/{tenant_slug}`), or one a handler reads through a helper — so every {name} in the path is
// in the operation, as OpenAPI requires.
func documentPathParams(op *huma.Operation, path string) {
	declared := map[string]bool{}
	for _, p := range op.Parameters {
		if p.In == "path" {
			declared[p.Name] = true
		}
	}
	for _, m := range reHumaParam.FindAllStringSubmatch(path, -1) {
		if !declared[m[1]] {
			op.Parameters = append(op.Parameters, &huma.Param{Name: m[1], In: "path", Required: true, Schema: &huma.Schema{Type: huma.TypeString}})
			declared[m[1]] = true
		}
	}
}
