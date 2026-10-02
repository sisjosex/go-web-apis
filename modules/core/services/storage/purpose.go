package storage

import (
	"sync"

	"github.com/google/uuid"
)

// Purpose is what an upload ticket is for: which bucket it lands in, which content types it may
// declare and how big it may be. A module registers its own; the ticket endpoint knows none of them.
type Purpose struct {
	Name  string
	Store ObjectStore
	// Types maps each content type the client may declare to the extension the key gets.
	Types    map[string]string
	MaxBytes int64
}

// PendingKey is where a ticket's upload lands until its owner claims it: under pending/, which the
// bucket's lifecycle rule sweeps after a day, and under the tenant, so a claim can check ownership
// from the key alone.
func PendingKey(tenantID uuid.UUID, ext string) string {
	return PendingPrefix(tenantID) + uuid.NewString() + "." + ext
}

// PendingPrefix is the part of a pending key that names its tenant.
func PendingPrefix(tenantID uuid.UUID) string {
	return "pending/" + tenantID.String() + "/"
}

// Purposes is the registry the ticket endpoint looks purposes up in.
type Purposes struct {
	mu     sync.RWMutex
	byName map[string]Purpose
}

func NewPurposes() *Purposes {
	return &Purposes{byName: map[string]Purpose{}}
}

func (p *Purposes) Register(purpose Purpose) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.byName[purpose.Name] = purpose
}

func (p *Purposes) Lookup(name string) (Purpose, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	purpose, ok := p.byName[name]
	return purpose, ok
}
