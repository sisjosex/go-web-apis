package services

import (
	importInterfaces "josex/web/modules/import/interfaces"
	importModels "josex/web/modules/import/models"
)

// registeredDescriptor is one descriptor and the module that registered it, so a
// scope can name either (IMPORT-001 D1).
type registeredDescriptor struct {
	module     string
	descriptor importInterfaces.ImportDescriptor
}

// registry is the concrete ImportRegistry: a resource-keyed lookup plus an
// insertion-ordered slice used for header-signature detection.
type registry struct {
	byResource map[string]importInterfaces.ImportDescriptor
	ordered    []registeredDescriptor
}

// NewRegistry creates an empty import registry.
func NewRegistry() importInterfaces.ImportRegistry {
	return &registry{
		byResource: make(map[string]importInterfaces.ImportDescriptor),
	}
}

func (r *registry) Register(module string, descriptor importInterfaces.ImportDescriptor) {
	r.byResource[descriptor.Resource()] = descriptor
	r.ordered = append(r.ordered, registeredDescriptor{module: module, descriptor: descriptor})
}

func (r *registry) Get(resource string) (importInterfaces.ImportDescriptor, bool) {
	descriptor, ok := r.byResource[resource]
	return descriptor, ok
}

// Resources lists the descriptors a scope reaches, in registration order. Each
// scope entry is a resource key or a module key; an empty scope reaches all.
func (r *registry) Resources(scope []string) []importModels.ImportResourceInfo {
	resources := []importModels.ImportResourceInfo{}
	for _, entry := range r.inScope(scope) {
		resources = append(resources, importModels.ImportResourceInfo{
			Resource: entry.descriptor.Resource(),
			Module:   entry.module,
		})
	}
	return resources
}

// Detect returns the first descriptor inside the scope whose header signature
// matches. Registration order breaks ties, so a scope never changes which of two
// matching descriptors wins — it only removes candidates.
func (r *registry) Detect(headers []string, scope []string) (importInterfaces.ImportDescriptor, bool) {
	for _, entry := range r.inScope(scope) {
		if entry.descriptor.Matches(headers) {
			return entry.descriptor, true
		}
	}
	return nil, false
}

// InScope reports whether a resource is one the scope reaches.
func (r *registry) InScope(resource string, scope []string) bool {
	for _, entry := range r.inScope(scope) {
		if entry.descriptor.Resource() == resource {
			return true
		}
	}
	return false
}

func (r *registry) inScope(scope []string) []registeredDescriptor {
	if len(scope) == 0 {
		return r.ordered
	}

	wanted := make(map[string]bool, len(scope))
	for _, entry := range scope {
		wanted[entry] = true
	}

	var reached []registeredDescriptor
	for _, entry := range r.ordered {
		if wanted[entry.module] || wanted[entry.descriptor.Resource()] {
			reached = append(reached, entry)
		}
	}
	return reached
}
