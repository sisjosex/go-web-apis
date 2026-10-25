package registry

import (
	"context"
	"slices"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SetupStep is one step of the first-run checklist (TENANCY-008): a module declares its own in its
// descriptor, the frontend Route that does it and about how many Minutes it takes. Shared steps
// (business, team) are tenancy's and frame every checklist.
type SetupStep struct {
	Code    string `json:"code"`
	Module  string `json:"module"`
	Minutes int    `json:"minutes"`
	Route   string `json:"route"`
	Shared  bool   `json:"shared,omitempty"`
}

// RowQuerier runs one statement on the request's database — the tenant's own when it has one.
type RowQuerier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// SetupChecker answers which of its module's steps a tenant has done, by code, in one SP call. A module
// registers it from init(); tenancy hands it the request's querier, so init() needs no database.
type SetupChecker func(ctx context.Context, q RowQuerier, tenantID uuid.UUID) (map[string]bool, error)

var setupCheckers = map[string]SetupChecker{}

// RegisterSetupChecker adds or replaces a module's checker. Called from the module's init().
func RegisterSetupChecker(module string, check SetupChecker) {
	mu.Lock()
	defer mu.Unlock()
	setupCheckers[module] = check
}

// SetupCheckerFor answers a module's checker.
func SetupCheckerFor(module string) (SetupChecker, bool) {
	mu.RLock()
	defer mu.RUnlock()
	check, ok := setupCheckers[module]
	return check, ok
}

// SetupSteps answers the enabled modules' steps in checklist order (D2): modules in the order of the
// business types' Modules — the first type's first —, then the rest by code; each module's steps as it
// declares them. The first type with a Setup list reorders them; a step it does not name keeps its
// place after the named ones.
func SetupSteps(verticalCodes []string, enabled map[string]bool) []SetupStep {
	mu.RLock()
	defer mu.RUnlock()
	order, override := moduleOrder(verticalCodes)
	var steps []SetupStep
	for _, module := range order {
		if !enabled[module] {
			continue
		}
		for _, step := range modules[module].Setup {
			step.Module = module
			steps = append(steps, step)
		}
	}
	if override != nil {
		rank := func(code string) int {
			if i := slices.Index(override, code); i >= 0 {
				return i
			}
			return len(override)
		}
		sort.SliceStable(steps, func(a, b int) bool { return rank(steps[a].Code) < rank(steps[b].Code) })
	}
	return steps
}

// moduleOrder answers every registered module, the business types' first in their order, and the first
// type's Setup override. The caller holds mu.
func moduleOrder(verticalCodes []string) (order, override []string) {
	for _, code := range verticalCodes {
		i := slices.IndexFunc(verticals, func(v Vertical) bool { return v.Code == code })
		if i < 0 {
			continue
		}
		if override == nil && len(verticals[i].Setup) > 0 {
			override = verticals[i].Setup
		}
		order = union(order, verticals[i].Modules)
	}
	rest := make([]string, 0, len(modules))
	for code := range modules {
		rest = append(rest, code)
	}
	sort.Strings(rest)
	return union(order, rest), override
}

func union(list, codes []string) []string {
	for _, code := range codes {
		if !slices.Contains(list, code) {
			list = append(list, code)
		}
	}
	return list
}
