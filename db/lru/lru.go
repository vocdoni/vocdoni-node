package lru

import (
	"sync"

	"github.com/hashicorp/golang-lru/simplelru"
)

// Cache implements a least-recently-used cache that is safe for concurrent use.
type Cache struct {
	lruMu sync.Mutex
	lru   simplelru.LRU
}

// New creates a new Cache with a given maximum number of entries.
func New(size int) *Cache {
	lru, err := simplelru.NewLRU(size, nil)
	if err != nil {
		// simplelru.NewLRU will only error if size is negative.
		// If that happens, panic, because our sizes should be static.
		// Arguably, their API should take an uint, or just panic too.
		panic(err)
	}
	return &Cache{lru: *lru}
}

type entry struct {
	valueReady sync.WaitGroup // while value is built
	value      interface{}
}

// GetOrAdd is a high-level method to be used with cache entries which are
// expensive to obtain from scratch.
//
// If the key is not in the cache, we insert a new entry into the cache, calling
// buildValue to obtain the expensive value to be stored. Note that the global
// cache mutex is not locked while buildValue is running.
//
// If the key is in the cache, we return the entry's inner value. If buildValue
// is still working for that entry, we wait for it to finish before returning.
func (l *Cache) GetOrAdd(key interface{}, buildValue func() interface{}) interface{} {
	l.lruMu.Lock()

	// If the value is in the cache, release the lock and return the value.
	if prev, ok := l.lru.Get(key); ok {
		l.lruMu.Unlock()

		// After we release the lock, wait for the value to be ready.
		entry := prev.(*entry)
		entry.valueReady.Wait()
		return entry.value
	}

	// Insert an empty entry with a waitgroup of 1,
	// and drop the lock.
	// If a GetOrAdd call grabs the entry while we're building the value,
	// they'll wait until our Done call below.
	entry := &entry{}
	entry.valueReady.Add(1)
	l.lru.Add(key, entry)
	l.lruMu.Unlock()

	// Once the value is ready, we can mark the waitgroup as done.
	entry.value = buildValue()
	entry.valueReady.Done()
	return entry.value
}
