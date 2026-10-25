package registry

import (
	"context"
	"encoding/json"
	"slices"
	"sort"
)

// SettingType is the shape of a tenant setting's value.
type SettingType string

const (
	SettingBool   SettingType = "bool"
	SettingString SettingType = "string"
	SettingEnum   SettingType = "enum"
	// SettingSet is a list of distinct options, as a JSON array of strings (TENANCY-006 D3).
	SettingSet SettingType = "set"
)

// SettingSpec declares one key of tenancy.tenants.settings (TENANCY-007 D1): the module that owns it,
// its type, the value it has until a tenant sets one and, for an enum, what it may be. A module
// declares its keys from its init(), so tenancy validates them without knowing any module.
type SettingSpec struct {
	Key     string      `json:"key"`
	Module  string      `json:"module"`
	Type    SettingType `json:"type"`
	Default any         `json:"default"`
	Options []string    `json:"options,omitempty"`
}

var settings = map[string]SettingSpec{}

// RegisterSetting adds or replaces a setting. Called from each module's init().
func RegisterSetting(s SettingSpec) {
	mu.Lock()
	defer mu.Unlock()
	settings[s.Key] = s
}

// SettingFor answers one setting by key.
func SettingFor(key string) (SettingSpec, bool) {
	mu.RLock()
	defer mu.RUnlock()
	s, ok := settings[key]
	return s, ok
}

// SettingsFor answers, sorted by key, the settings of the modules a tenant has on. A setting whose
// module is not an installable one (tenancy's own) is always offered.
func SettingsFor(enabled map[string]bool) []SettingSpec {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]SettingSpec, 0, len(settings))
	for _, s := range settings {
		if _, installable := modules[s.Module]; !installable || enabled[s.Module] {
			out = append(out, s)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Key < out[b].Key })
	return out
}

// Accepts reports whether v is a value this setting may hold, as it arrives decoded from JSON.
func (s SettingSpec) Accepts(v any) bool {
	switch s.Type {
	case SettingBool:
		_, ok := v.(bool)
		return ok
	case SettingString:
		_, ok := v.(string)
		return ok
	case SettingEnum:
		str, ok := v.(string)
		return ok && slices.Contains(s.Options, str)
	case SettingSet:
		return s.set(v) != nil
	}
	return false
}

// set answers v as the distinct options it lists, or nil when it is not such a list.
func (s SettingSpec) set(v any) []string {
	items, ok := v.([]any)
	if !ok {
		if strs, isStrings := v.([]string); isStrings {
			items = make([]any, len(strs))
			for i, str := range strs {
				items[i] = str
			}
		} else {
			return nil
		}
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		str, isString := item.(string)
		if !isString || !slices.Contains(s.Options, str) || slices.Contains(out, str) {
			return nil
		}
		out = append(out, str)
	}
	return out
}

// Value is the stored value when it is a valid one, else the default.
func (s SettingSpec) Value(stored map[string]any) any {
	if v, ok := stored[s.Key]; ok && s.Accepts(v) {
		if s.Type == SettingSet {
			return s.set(v)
		}
		return v
	}
	return s.Default
}

// ParseSettings decodes tenancy.tenants.settings; NULL or malformed is no setting at all.
func ParseSettings(raw *string) map[string]any {
	stored := map[string]any{}
	if raw != nil {
		_ = json.Unmarshal([]byte(*raw), &stored)
	}
	return stored
}

// SettingsReader answers a tenant's stored settings by slug. Tenancy installs one that reads through
// the shared tenant cache, so a request-path read costs no query while it is warm (D1).
type SettingsReader func(ctx context.Context, slug string) (map[string]any, error)

var settingsReader SettingsReader

// UseSettingsReader installs the reader; without one every setting reads its default.
func UseSettingsReader(r SettingsReader) {
	mu.Lock()
	defer mu.Unlock()
	settingsReader = r
}

// TenantSetting answers a tenant's value for key: its own when set and valid, else the default. A
// failed read answers the default too — a setting never fails the request that reads it.
func TenantSetting(ctx context.Context, slug, key string) any {
	mu.RLock()
	spec, ok := settings[key]
	read := settingsReader
	mu.RUnlock()
	if !ok {
		return nil
	}
	if read == nil || slug == "" {
		return spec.Default
	}
	stored, err := read(ctx, slug)
	if err != nil {
		return spec.Default
	}
	return spec.Value(stored)
}

// SettingOn is a bool setting read from a tenant's stored settings — what a background job holds from
// its in-memory directory (D2). An unknown key is on.
func SettingOn(stored map[string]any, key string) bool {
	spec, ok := SettingFor(key)
	if !ok {
		return true
	}
	on, _ := spec.Value(stored).(bool)
	return on
}
