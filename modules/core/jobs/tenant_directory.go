package jobs

import (
	"context"
	"sort"
	"sync"
	"time"
)

// directoryRefreshEvery is how stale the directory may get: a new tenant is walked within it, and an
// unknown id met before then reloads it at once (INFRA-009 D5).
const directoryRefreshEvery = 60 * time.Second

// directoryMissEvery caps the reloads an unknown id may force, so a stream of tasks for a deleted
// tenant costs one platform query per second, not one per task.
const directoryMissEvery = time.Second

// Database is one database the workers drain: the shared platform one (URL "", INFRA-006 D2) or a
// tenant's own. Its tenants are the ones whose rows it holds.
type Database struct {
	URL     string
	Tenants []Tenant
}

// Shared reports whether this is the platform database every shared tenant lives in.
func (d Database) Shared() bool {
	return d.URL == ""
}

// TenantDirectory is every tenant the workers serve, read from the platform database and kept in
// memory: the relay, the GPS consumer and every outbox handler resolve a tenant here, never with a
// query of their own (INFRA-009 D5).
type TenantDirectory struct {
	list TenantLister

	mu       sync.Mutex
	tenants  []Tenant
	byID     map[string]Tenant
	loadedAt time.Time
	missAt   time.Time
}

func NewTenantDirectory(list TenantLister) *TenantDirectory {
	return &TenantDirectory{list: list}
}

// List answers every tenant, reloading when the copy held is older than directoryRefreshEvery. A
// failed reload keeps the last good copy when there is one.
func (d *TenantDirectory) List(ctx context.Context) ([]Tenant, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.byID == nil || time.Since(d.loadedAt) > directoryRefreshEvery {
		if err := d.reload(ctx); err != nil && d.byID == nil {
			return nil, err
		}
	}
	return d.tenants, nil
}

// Find answers one tenant by id. An id the copy does not hold reloads it — a tenant created a moment
// ago is found on its first task — at most once per directoryMissEvery.
func (d *TenantDirectory) Find(ctx context.Context, id string) (Tenant, bool, error) {
	if _, err := d.List(ctx); err != nil {
		return Tenant{}, false, err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if t, ok := d.byID[id]; ok {
		return t, true, nil
	}
	if time.Since(d.missAt) < directoryMissEvery {
		return Tenant{}, false, nil
	}
	d.missAt = time.Now()
	if err := d.reload(ctx); err != nil {
		return Tenant{}, false, err
	}
	t, ok := d.byID[id]
	return t, ok, nil
}

// Databases groups the tenants by the database they live in, the shared one first.
func (d *TenantDirectory) Databases(ctx context.Context) ([]Database, error) {
	tenants, err := d.List(ctx)
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	var dbs []Database
	for _, t := range tenants {
		i, ok := index[t.DatabaseURL]
		if !ok {
			i = len(dbs)
			index[t.DatabaseURL] = i
			dbs = append(dbs, Database{URL: t.DatabaseURL})
		}
		dbs[i].Tenants = append(dbs[i].Tenants, t)
	}
	sort.SliceStable(dbs, func(a, b int) bool { return dbs[a].Shared() && !dbs[b].Shared() })
	return dbs, nil
}

func (d *TenantDirectory) reload(ctx context.Context) error {
	tenants, err := d.list(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]Tenant, len(tenants))
	for _, t := range tenants {
		byID[t.ID] = t
	}
	d.tenants, d.byID, d.loadedAt = tenants, byID, time.Now()
	return nil
}
