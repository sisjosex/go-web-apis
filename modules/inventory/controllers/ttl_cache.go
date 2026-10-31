package controllers

import (
	"sync"
	"time"
)

// ttlCacheMaxEntries keeps a cache of per-query answers from growing without bound; full, it starts over.
const ttlCacheMaxEntries = 1000

type ttlEntry[T any] struct {
	value   T
	expires time.Time
}

// ttlCache keeps answers that are read on every view and only move on a write (facets, the inventory home).
type ttlCache[T any] struct {
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]ttlEntry[T]
}

func newTTLCache[T any](ttl time.Duration) *ttlCache[T] {
	return &ttlCache[T]{ttl: ttl, entries: map[string]ttlEntry[T]{}}
}

func (c *ttlCache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok || time.Now().After(entry.expires) {
		var zero T
		return zero, false
	}
	return entry.value, true
}

func (c *ttlCache[T]) set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= ttlCacheMaxEntries {
		c.entries = map[string]ttlEntry[T]{}
	}
	c.entries[key] = ttlEntry[T]{value: value, expires: time.Now().Add(c.ttl)}
}
