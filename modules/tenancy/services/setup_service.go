package services

import (
	"context"

	"josex/web/modules/core/registry"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// The checklist's shared steps (TENANCY-008): the business's data first, the team last.
var (
	setupBusiness = registry.SetupStep{Code: "business", Module: "tenancy", Minutes: 2, Route: "/settings", Shared: true}
	setupTeam     = registry.SetupStep{Code: "team", Module: "tenancy", Minutes: 3, Route: "/users", Shared: true}
)

// TenantSetup builds the checklist from the enabled modules, ordered by the stored business types (D2).
// One tenancy SP plus one SP per enabled module that has steps; the module list comes from the cache.
func (s *moduleService) TenantSetup(ctx context.Context, tenantID uuid.UUID) (*models.TenantSetupResponse, error) {
	business, team, verticals, err := s.moduleRepository.TenantSetup(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	enabled, err := s.GetTenantEnabledModuleCodes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	done := map[string]map[string]bool{"tenancy": {"business": business, "team": team}}
	steps := append(append([]registry.SetupStep{setupBusiness}, registry.SetupSteps(verticals, enabled)...), setupTeam)
	for _, step := range steps {
		if _, read := done[step.Module]; read {
			continue
		}
		check, ok := registry.SetupCheckerFor(step.Module)
		if !ok {
			done[step.Module] = map[string]bool{}
			continue
		}
		if done[step.Module], err = s.moduleRepository.CheckSetup(ctx, check, tenantID); err != nil {
			return nil, err
		}
	}

	out := &models.TenantSetupResponse{Steps: make([]models.SetupStepStatus, 0, len(steps)), Total: len(steps)}
	for _, step := range steps {
		ok := done[step.Module][step.Code]
		out.Steps = append(out.Steps, models.SetupStepStatus{Code: step.Code, Module: step.Module, Done: ok, Minutes: step.Minutes, Route: step.Route})
		if ok {
			out.Done++
		} else {
			out.MinutesLeft += step.Minutes
		}
	}
	return out, nil
}
