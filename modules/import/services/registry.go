package services

import (
	importInterfaces "josex/web/modules/import/interfaces"
)

// registry is the concrete ImportRegistry: a resource-keyed lookup plus an
// insertion-ordered slice used for header-signature detection.
type registry struct {
	byResource map[string]importInterfaces.ImportDescriptor
	ordered    []importInterfaces.ImportDescriptor
}

// NewRegistry creates an empty import registry.
func NewRegistry() importInterfaces.ImportRegistry {
	return &registry{
		byResource: make(map[string]importInterfaces.ImportDescriptor),
	}
}

func (r *registry) Register(descriptor importInterfaces.ImportDescriptor) {
	r.byResource[descriptor.Resource()] = descriptor
	r.ordered = append(r.ordered, descriptor)
}

func (r *registry) Get(resource string) (importInterfaces.ImportDescriptor, bool) {
	descriptor, ok := r.byResource[resource]
	return descriptor, ok
}

// Detect returns the first registered descriptor whose header signature matches.
// Detection is header-based in this iteration; filename-rule and ZIP strategies
// are future additions and would plug in here without changing callers.
func (r *registry) Detect(headers []string) (importInterfaces.ImportDescriptor, bool) {
	var match importInterfaces.ImportDescriptor
	found := false
	for _, descriptor := range r.ordered {
		if !found && descriptor.Matches(headers) {
			match = descriptor
			found = true
		}
	}
	return match, found
}
