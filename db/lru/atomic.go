package lru

import (
	"sync"

	"github.com/hashicorp/golang-lru/v2/simplelru"
)

// AtomicCache implements a least-recently-used cache that is safe for concurrent use.
// All operations are atomic.
type AtomicCache[K comparable, V any] struct {
	lruMu sync.Mutex
	lru   simplelru.LRU[K, *entry[V]]
}

// NewAtomic creates a new Cache with a given maximum number of entries.
func NewAtomic[K comparable, V any](size int) *AtomicCache[K, V] {
	lru, err := simplelru.NewLRU[K, *entry[V]](size, nil)
	if err != nil {
		// simplelru.NewLRU will only error if size is negative.
		// If that happens, panic, because our sizes should be static.
		// Arguably, their API should take an uint, or just panic too.
		panic(err)
	}
	return &AtomicCache[K, V]{lru: *lru}
}

type entry[V any] struct {
	valueMu sync.Mutex // while value is built or updated
	value   V
}

// GetAndUpdate is a high-level method to be used with cache entries which are
// expensive to obtain from scratch.
//
// If the key is not in the cache, we insert a new entry into the cache, calling
// updateValue(zeroValue) to obtain the expensive value to be stored.
// Note that the global cache mutex is not locked while updateValue is running.
//
// If the key is in the cache, we update its cached value by calling updateValue
// on it, and return the new value. If updateValue was already working for that
// entry, we wait for it to finish first.
//
// If existing entries shouldn't be updated, one can use an updateValue func
// that simply returns its input parameter when it's non-nil.
func (l *AtomicCache[K, V]) GetAndUpdate(key K, updateValue func(V) V) V {
	l.lruMu.Lock()

	// If the value is in the cache, release the lock and return the value.
	if entry, ok := l.lru.Get(key); ok {
		l.lruMu.Unlock()

		// After we release lruMu, wait for the value to be ready.
		entry.valueMu.Lock()
		defer entry.valueMu.Unlock()

		entry.value = updateValue(entry.value)

		return entry.value
	}

	// Insert an empty entry with a grabbed valueMu, and drop lruMu.
	// If a GetOrUpdate call grabs the entry while we're building the value,
	// they'll wait until our Done call below.
	entry := &entry[V]{}
	entry.valueMu.Lock()
	defer entry.valueMu.Unlock()
	l.lru.Add(key, entry)
	l.lruMu.Unlock()

	// Once the value is ready, we can release valueMu.
	var zeroV V // e.g. nil
	entry.value = updateValue(zeroV)
	return entry.value
}
