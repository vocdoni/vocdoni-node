package lru

import (
	"sync"
	"time"
)

// NewTTLCache creates a new TTL cache for the given types.
func NewTTLCache[K comparable, V any]() *TTLCache[K, V] {
	return &TTLCache[K, V]{entries: make(map[K]ttlEntry[V])}
}

// TTLCache implements a safe cache where each entry has an expiration time.
// Note that the API doesn't use [time.Duration] nor [time.Now];
// this allows the caller to reuse a single [time.Now] call when inserting
// many entries, and enables testing without any sleeps.
type TTLCache[K comparable, V any] struct {
	entriesMu sync.RWMutex
	entries   map[K]ttlEntry[V]
}

type ttlEntry[V any] struct {
	expiration time.Time
	value      V
}

// Put inserts a new element to the cache, with the given expiration time.
func (c *TTLCache[K, V]) Put(k K, v V, expiration time.Time) {
	entry := ttlEntry[V]{expiration: expiration, value: v}

	c.entriesMu.Lock()
	defer c.entriesMu.Unlock()
	c.entries[k] = entry
}

// Get retrieves an element from the cache.
func (c *TTLCache[K, V]) Get(k K) (V, bool) {
	c.entriesMu.RLock()
	defer c.entriesMu.RUnlock()

	entry, ok := c.entries[k]
	return entry.value, ok
}

// Evict removes all entries that have expired before the given time.
func (c *TTLCache[K, V]) Evict(until time.Time) uint32 {
	c.entriesMu.Lock()
	defer c.entriesMu.Unlock()
	deleteCount := uint32(0)
	// TODO: if we end up with tens of thousands of entries in the cache,
	// this linear search may become somewhat inefficient.
	// We could consider maintaining a sorted list of expiration times.
	for k, entry := range c.entries {
		if entry.expiration.Before(until) {
			delete(c.entries, k)
			deleteCount++
		}
	}
	return deleteCount
}
