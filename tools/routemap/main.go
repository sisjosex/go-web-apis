// Command routemap writes docs/routes.json (INFRA-021): every operation of docs/openapi.json with
// the stored procedures its handler reaches and the tables those read and write. The system map in
// spec-agent joins it with the app's and the phone's calls.
//
//	go run ./tools/routemap          # write docs/routes.json
//	go run ./tools/routemap -check   # fail when it is stale
//
// Reach is static: from the handler, every call the type checker resolves, an interface method
// standing for each implementation of it in the module tree, until a repository's SQL. SPs are the
// `schema.sp_x(` / `schema.fn_x(` names in the reached functions' string literals; tables come from
// each function's sql/functions file, the functions it calls included.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	openapiPath = "docs/openapi.json"
	routesPath  = "docs/routes.json"
	modulePath  = "josex/web/"
)

// Route is one line of routes.json.
type Route struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	OperationID string   `json:"operation_id"`
	Module      string   `json:"module"`
	Handler     string   `json:"handler"`
	SPs         []string `json:"sps"`
	Reads       []string `json:"reads"`
	Writes      []string `json:"writes"`
}

// Function is one SQL function file: what it reads, writes and calls.
type Function struct {
	Reads  []string `json:"reads"`
	Writes []string `json:"writes"`
	Calls  []string `json:"calls"`
}

// Map is routes.json.
type Map struct {
	Routes    []Route             `json:"routes"`
	Functions map[string]Function `json:"functions"`
}

var (
	reSP     = regexp.MustCompile(`\b([a-z_]+)\.((?:sp|fn)_[a-z0-9_]+)\s*\(`)
	reWrite  = regexp.MustCompile(`(?i)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM|MERGE\s+INTO)\s+([a-z_]+)\.([a-z_][a-z0-9_]*)`)
	reRead   = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-z_]+)\.([a-z_][a-z0-9_]*)`)
	reFnHead = regexp.MustCompile(`(?i)CREATE\s+OR\s+REPLACE\s+FUNCTION\s+([a-z_]+)\.([a-z0-9_]+)`)
	reLineC  = regexp.MustCompile(`--[^\n]*`)
)

func main() {
	check := flag.Bool("check", false, "fail when docs/routes.json is stale")
	flag.Parse()

	ops, err := readOperations()
	if err != nil {
		fail(err)
	}
	functions, err := readFunctions()
	if err != nil {
		fail(err)
	}
	reach, err := handlerSPs(ops)
	if err != nil {
		fail(err)
	}

	m := Map{Functions: functions}
	for _, op := range ops {
		r := op
		sps := reach[op.Handler]
		r.SPs = sps
		reads, writes := map[string]bool{}, map[string]bool{}
		seen := map[string]bool{}
		var walk func(fn string)
		walk = func(fn string) {
			if seen[fn] {
				return
			}
			seen[fn] = true
			f := functions[fn]
			for _, t := range f.Reads {
				reads[t] = true
			}
			for _, t := range f.Writes {
				writes[t] = true
			}
			for _, c := range f.Calls {
				walk(c)
			}
		}
		for _, sp := range sps {
			walk(sp)
		}
		r.Reads, r.Writes = sorted(reads), sorted(writes)
		m.Routes = append(m.Routes, r)
	}

	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		fail(err)
	}
	out = append(out, '\n')
	if *check {
		committed, err := os.ReadFile(routesPath)
		if err != nil || !bytes.Equal(bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")), out) {
			fail(fmt.Errorf("%s is stale — run make contract and commit it", routesPath))
		}
		return
	}
	if err := os.WriteFile(routesPath, out, 0o644); err != nil {
		fail(err)
	}
	unmapped := 0
	for _, r := range m.Routes {
		if len(r.SPs) == 0 {
			unmapped++
		}
	}
	fmt.Printf("routes.json: %d routes, %d with no stored procedure, %d functions\n", len(m.Routes), unmapped, len(functions))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "routemap:", err)
	os.Exit(1)
}

func sorted(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// readOperations lists the contract's operations with the x-handler the API stamps on each.
func readOperations() ([]Route, error) {
	raw, err := os.ReadFile(openapiPath)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Paths map[string]map[string]struct {
			OperationID string   `json:"operationId"`
			Tags        []string `json:"tags"`
			Handler     string   `json:"x-handler"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var out []Route
	for path, item := range doc.Paths {
		for method, op := range item {
			module := ""
			if m := regexp.MustCompile(`/modules/([a-z_]+)/`).FindStringSubmatch(op.Handler); m != nil {
				module = m[1]
			}
			out = append(out, Route{Method: strings.ToUpper(method), Path: path, OperationID: op.OperationID, Module: module, Handler: op.Handler})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out, nil
}

// readFunctions parses every modules/*/sql/functions/*.sql.
func readFunctions() (map[string]Function, error) {
	files, err := filepath.Glob("modules/*/sql/functions/*.sql")
	if err != nil {
		return nil, err
	}
	out := map[string]Function{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		sql := reLineC.ReplaceAllString(string(raw), "")
		head := reFnHead.FindStringSubmatch(sql)
		if head == nil {
			continue
		}
		name := strings.ToLower(head[1] + "." + head[2])
		reads, writes, calls := map[string]bool{}, map[string]bool{}, map[string]bool{}
		for _, m := range reWrite.FindAllStringSubmatch(sql, -1) {
			writes[strings.ToLower(m[1]+"."+m[2])] = true
		}
		for _, m := range reRead.FindAllStringSubmatch(sql, -1) {
			t := strings.ToLower(m[1] + "." + m[2])
			if !strings.Contains(t, ".sp_") && !strings.Contains(t, ".fn_") {
				reads[t] = true
			}
		}
		for _, m := range reSP.FindAllStringSubmatch(sql, -1) {
			fn := strings.ToLower(m[1] + "." + m[2])
			if fn != name {
				calls[fn] = true
			}
		}
		// DELETE FROM x also matched FROM x as a read
		for t := range writes {
			delete(reads, t)
		}
		out[name] = Function{Reads: sorted(reads), Writes: sorted(writes), Calls: sorted(calls)}
	}
	return out, nil
}

// handlerSPs follows each handler's calls and answers the SPs named in what it reaches.
func handlerSPs(ops []Route) (map[string][]string, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedFiles | packages.NeedImports | packages.NeedDeps, Fset: fset}
	pkgs, err := packages.Load(cfg, "./modules/...")
	if err != nil {
		return nil, err
	}

	type fnInfo struct {
		decl *ast.FuncDecl
		pkg  *packages.Package
	}
	decls := map[*types.Func]fnInfo{}
	// package-level variables holding SQL (a map of statements by report, say): their SPs
	varSPs := map[*types.Var]map[string]bool{}
	byName := map[string]*types.Func{}
	var named []*types.Named
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if !strings.HasPrefix(p.PkgPath, modulePath) {
			return
		}
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.VAR {
					for _, spec := range gd.Specs {
						vs := spec.(*ast.ValueSpec)
						found := map[string]bool{}
						for _, v := range vs.Values {
							collectSPs(v, found)
						}
						if len(found) == 0 {
							continue
						}
						for _, name := range vs.Names {
							if obj, ok := p.TypesInfo.Defs[name].(*types.Var); ok {
								varSPs[obj] = found
							}
						}
					}
				}
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				if obj, ok := p.TypesInfo.Defs[fd.Name].(*types.Func); ok {
					decls[obj] = fnInfo{fd, p}
					byName[obj.FullName()] = obj
				}
			}
		}
		for _, n := range p.Types.Scope().Names() {
			if tn, ok := p.Types.Scope().Lookup(n).(*types.TypeName); ok {
				if nt, ok := tn.Type().(*types.Named); ok {
					if _, isIface := nt.Underlying().(*types.Interface); !isIface {
						named = append(named, nt)
					}
				}
			}
		}
	})

	// implementations of an interface method, by the concrete types of the module tree
	impls := map[*types.Func][]*types.Func{}
	implOf := func(m *types.Func) []*types.Func {
		if got, ok := impls[m]; ok {
			return got
		}
		recv := m.Type().(*types.Signature).Recv()
		var out []*types.Func
		if recv != nil {
			if iface, ok := recv.Type().Underlying().(*types.Interface); ok {
				for _, nt := range named {
					for _, t := range []types.Type{nt, types.NewPointer(nt)} {
						if types.Implements(t, iface) {
							obj, _, _ := types.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
							if f, ok := obj.(*types.Func); ok {
								out = append(out, f)
							}
							break
						}
					}
				}
			}
		}
		impls[m] = out
		return out
	}

	memo := map[*types.Func]map[string]bool{}
	var reachSPs func(fn *types.Func, stack map[*types.Func]bool) map[string]bool
	reachSPs = func(fn *types.Func, stack map[*types.Func]bool) map[string]bool {
		if got, ok := memo[fn]; ok {
			return got
		}
		if stack[fn] {
			return nil
		}
		stack[fn] = true
		defer delete(stack, fn)
		out := map[string]bool{}
		info, ok := decls[fn]
		if !ok {
			return out
		}
		ast.Inspect(info.decl.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BasicLit:
				collectSPs(x, out)
			case *ast.Ident:
				if v, ok := info.pkg.TypesInfo.Uses[x].(*types.Var); ok {
					for sp := range varSPs[v] {
						out[sp] = true
					}
				}
			case *ast.CallExpr:
				var callee *types.Func
				fun := x.Fun
				switch g := fun.(type) { // report[T](…): the instantiated name
				case *ast.IndexExpr:
					fun = g.X
				case *ast.IndexListExpr:
					fun = g.X
				}
				switch f := fun.(type) {
				case *ast.Ident:
					callee, _ = info.pkg.TypesInfo.Uses[f].(*types.Func)
				case *ast.SelectorExpr:
					if sel := info.pkg.TypesInfo.Selections[f]; sel != nil {
						callee, _ = sel.Obj().(*types.Func)
					} else {
						callee, _ = info.pkg.TypesInfo.Uses[f.Sel].(*types.Func)
					}
				}
				if callee == nil || callee.Pkg() == nil || !strings.HasPrefix(callee.Pkg().Path(), modulePath) {
					return true
				}
				callee = callee.Origin() // a generic function's instance is declared once, generically
				targets := []*types.Func{callee}
				if _, concrete := decls[callee]; !concrete {
					targets = implOf(callee)
				}
				for _, t := range targets {
					for sp := range reachSPs(t, stack) {
						out[sp] = true
					}
				}
			}
			return true
		})
		memo[fn] = out
		return out
	}

	result := map[string][]string{}
	for _, op := range ops {
		fn := byName[handlerFullName(op.Handler)]
		if fn == nil {
			continue
		}
		result[op.Handler] = sorted(reachSPs(fn, map[*types.Func]bool{}))
	}
	return result, nil
}

// handlerFullName turns runtime's "josex/web/modules/x/controllers.(*T).M" into go/types' FullName
// "(*josex/web/modules/x/controllers.T).M".
func handlerFullName(h string) string {
	for _, open := range []string{".(*", ".("} {
		if pkg, rest, ok := strings.Cut(h, open); ok {
			typ, method, _ := strings.Cut(rest, ")")
			return "(" + strings.TrimPrefix(open, ".(") + pkg + "." + typ + ")" + method
		}
	}
	return h
}

// collectSPs adds the SP names in n's string literals.
func collectSPs(n ast.Node, out map[string]bool) {
	ast.Inspect(n, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			s, _ := strconv.Unquote(lit.Value)
			for _, m := range reSP.FindAllStringSubmatch(s, -1) {
				out[strings.ToLower(m[1]+"."+m[2])] = true
			}
		}
		return true
	})
}
