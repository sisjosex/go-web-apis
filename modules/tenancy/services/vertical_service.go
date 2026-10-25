package services

import (
	"context"
	"slices"
	"sort"

	"josex/web/config"
	"josex/web/modules/core/registry"
	"josex/web/modules/core/utils"
	tenancyErrors "josex/web/modules/tenancy/errors"
	"josex/web/modules/tenancy/models"

	"github.com/google/uuid"
)

// verticalPreset is what a set of business types turns on: the union of their modules (requirements
// first), capabilities and starter categories, and the main module the first type defaults to.
type verticalPreset struct {
	picked       []string
	modules      []string
	capabilities []string
	starter      []string
	fallback     string
}

// SetVerticals applies a business type preset in one request (TENANCY-006 D2-D4). It never turns a
// module off: modules already on keep their config, the rest of the union is enabled with its
// requirements. Registry lookups, one module read, one transaction and, with Starter, one seed call.
func (s *moduleService) SetVerticals(ctx context.Context, tenantID uuid.UUID, slug string, dto models.SetVerticalsDto, lang string, enabledBy uuid.UUID) (*models.VerticalsResponse, error) {
	if len(dto.Codes) == 0 && dto.PrimaryModule == "" {
		return nil, &tenancyErrors.VerticalError{Code: tenancyErrors.VerticalRequired}
	}
	preset, err := resolvePreset(dto)
	if err != nil {
		return nil, err
	}
	enabled, err := s.enabledCodes(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	missing, after := withModules(enabled, preset.modules)
	if dto.PrimaryModule != "" && !slices.Contains(after, dto.PrimaryModule) {
		return nil, &tenancyErrors.VerticalError{Code: tenancyErrors.VerticalPrimaryNotEnabled}
	}

	defer InvalidateTenantModules(tenantID)
	verticals, primary, settings, err := s.moduleRepository.ApplyVerticals(ctx, tenantID, models.VerticalsWrite{
		Missing:       missing,
		Verticals:     preset.picked,
		PrimaryModule: dto.PrimaryModule,
		Fallback:      preset.fallback,
		Capabilities:  preset.capabilities,
		Restrict:      dto.Starter,
	}, enabledBy)
	if err != nil {
		return nil, err
	}
	forgetTenant(ctx, slug)

	seeded := 0
	if dto.Starter && slices.Contains(after, "inventory") && len(preset.starter) > 0 {
		if seeded, err = s.moduleRepository.SeedCategories(ctx, tenantID, starterNames(preset.starter, lang)); err != nil {
			return nil, err
		}
	}

	on := []string{}
	if spec, ok := registry.SettingFor(registry.CapabilitiesSetting); ok {
		on, _ = spec.Value(registry.ParseSettings(settings)).([]string)
	}
	return &models.VerticalsResponse{Verticals: verticals, PrimaryModule: primary, Enabled: after, Capabilities: on, SeededCategories: seeded}, nil
}

// resolvePreset checks every code against the types this deployment offers (D1) and every extra module
// against what they suggest, and answers their union.
func resolvePreset(dto models.SetVerticalsDto) (verticalPreset, error) {
	offered := map[string]registry.Vertical{}
	for _, v := range registry.OfferedVerticals(config.ModularAppConfig.Core.IsModuleEnabled) {
		offered[v.Code] = v
	}
	var p verticalPreset
	var suggested []string
	for _, code := range dto.Codes {
		v, ok := offered[code]
		if !ok {
			return p, &tenancyErrors.VerticalError{Code: tenancyErrors.VerticalUnknown}
		}
		if slices.Contains(p.picked, code) {
			continue
		}
		if len(p.picked) == 0 {
			p.fallback = v.Primary()
		}
		p.picked = append(p.picked, code)
		for _, module := range v.Modules {
			p.modules = union(p.modules, registry.WithRequirements(module))
		}
		suggested = union(suggested, v.Suggested)
		p.capabilities = union(p.capabilities, v.Capabilities)
		p.starter = union(p.starter, v.Starter)
	}
	for _, module := range dto.ExtraModules {
		if !slices.Contains(suggested, module) {
			return p, &tenancyErrors.VerticalError{Code: tenancyErrors.VerticalNotSuggested}
		}
		p.modules = union(p.modules, registry.WithRequirements(module))
	}
	return p, nil
}

// withModules answers, of modules, those still off in order, and every module on once they are, sorted.
func withModules(enabled map[string]bool, modules []string) (missing, after []string) {
	for code := range enabled {
		after = append(after, code)
	}
	for _, code := range modules {
		if !enabled[code] && !slices.Contains(missing, code) {
			missing = append(missing, code)
			after = append(after, code)
		}
	}
	sort.Strings(after)
	return missing, after
}

// starterNames are the starter categories in lang, the registry's English where lang/ has none.
func starterNames(codes []string, lang string) []string {
	names := make([]string, 0, len(codes))
	for _, code := range codes {
		key := "category.starter." + code
		name := utils.GetTranslation(lang, key)
		if name == key {
			name = registry.StarterNames[code]
		}
		names = append(names, name)
	}
	return names
}

func union(list, codes []string) []string {
	for _, code := range codes {
		if !slices.Contains(list, code) {
			list = append(list, code)
		}
	}
	return list
}
