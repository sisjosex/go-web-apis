package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// The -routes and -modules modes (INFRA-014): endpoint coverage from the test recorder, and which
// modules a change must test.

var (
	reHandlerModule = regexp.MustCompile(`josex/web/modules/([a-z_]+)/`)
	reTestTarget    = regexp.MustCompile(`(?m)^test-([a-z]+):`)
)

// sharedPrefixes are the paths every module depends on: a change under one tests them all (D2).
var sharedPrefixes = []string{
	"modules/core/", "modules/auth/", "modules/tenancy/", "modules/users/",
	"routes/", "cmd/", "config/", "go.mod", "go.sum",
}

// routeUse is what the tests did with one route: every status they got, and whether one was a 2xx.
type routeUse struct {
	module   string
	statuses map[string]bool
	ok       bool
}

// checkRoutes reads the file the test recorder wrote (modules/core/testhelpers, ROUTE_HITS) and fails
// on a test request no route answered, or on a route of the given modules no test reached with a 2xx
// (INFRA-014 D1). The route's module is its handler's package, as routes/ registered it.
func checkRoutes(path string, modules []string) int {
	routes, unmatched, err := readRouteHits(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "check -routes:", err)
		return 2
	}
	wanted := map[string]bool{}
	for _, m := range modules {
		wanted[m] = true
	}

	var gaps []string
	for key, use := range routes {
		if !wanted[use.module] || use.ok {
			continue
		}
		gaps = append(gaps, key+" — "+reached(use))
	}
	sort.Strings(gaps)
	sort.Strings(unmatched)

	for _, u := range unmatched {
		fmt.Println("unmatched test request (no route answers it — wrong method or path): " + u)
	}
	for _, g := range gaps {
		fmt.Println("route without a 2xx test hit: " + g)
	}
	if len(gaps)+len(unmatched) > 0 {
		fmt.Printf("check -routes %s — %d route(s) without a 2xx hit, %d unmatched request(s)\n", strings.Join(modules, " "), len(gaps), len(unmatched))
		return 1
	}
	fmt.Printf("check -routes %s — every route reached with a 2xx\n", strings.Join(modules, " "))
	return 0
}

func reached(use *routeUse) string {
	if len(use.statuses) == 0 {
		return "never reached"
	}
	var statuses []string
	for s := range use.statuses {
		statuses = append(statuses, s)
	}
	sort.Strings(statuses)
	return "reached with " + strings.Join(statuses, ", ")
}

// readRouteHits parses "route METHOD path handler" and "hit METHOD route status path" lines. Every
// test binary writes the route lines, so they repeat.
func readRouteHits(path string) (map[string]*routeUse, []string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w — run the module's tests through make, which sets ROUTE_HITS", err)
	}

	routes := map[string]*routeUse{}
	var hits [][]string
	for line := range strings.SplitSeq(string(raw), "\n") {
		fields := strings.Fields(line)
		switch {
		case len(fields) == 4 && fields[0] == "route":
			if m := reHandlerModule.FindStringSubmatch(fields[3]); m != nil {
				routes[fields[1]+" "+fields[2]] = &routeUse{module: m[1], statuses: map[string]bool{}}
			}
		case len(fields) == 5 && fields[0] == "hit":
			hits = append(hits, fields)
		}
	}

	seen := map[string]bool{}
	var unmatched []string
	for _, h := range hits {
		if h[2] == "-" {
			if u := h[1] + " " + h[4] + " (" + h[3] + ")"; !seen[u] {
				seen[u] = true
				unmatched = append(unmatched, u)
			}
			continue
		}
		if use := routes[h[1]+" "+h[2]]; use != nil {
			use.statuses[h[3]] = true
			use.ok = use.ok || strings.HasPrefix(h[3], "2")
		}
	}
	return routes, unmatched, nil
}

// changedModules answers the modules whose tests a change needs: the module of each changed file, or
// every module with a test target when shared code changed (D2).
func changedModules() ([]string, error) {
	makefile, err := os.ReadFile("Makefile")
	if err != nil {
		return nil, err
	}
	var all []string
	for _, m := range reTestTarget.FindAllStringSubmatch(string(makefile), -1) {
		if m[1] != "all" {
			all = append(all, m[1])
		}
	}

	changed, err := git("diff", "--name-only", "HEAD")
	if err != nil {
		return nil, err
	}
	untracked, err := git("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}

	picked := map[string]bool{}
	for _, f := range append(changed, untracked...) {
		f = strings.ReplaceAll(f, "\\", "/")
		for _, prefix := range sharedPrefixes {
			if strings.HasPrefix(f, prefix) {
				return all, nil
			}
		}
		if rest, ok := strings.CutPrefix(f, "modules/"); ok {
			picked[strings.SplitN(rest, "/", 2)[0]] = true
		}
	}
	var out []string
	for _, m := range all {
		if picked[m] {
			out = append(out, m)
		}
	}
	return out, nil
}
