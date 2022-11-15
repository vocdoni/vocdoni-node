package lru

import (
	glru "github.com/hashicorp/golang-lru"
)

// Cache implements a least-recently-used cache that is safe for concurrent use.
type Cache struct {
	lru *glru.Cache
}

// New creates a new LRU Cache with a given maximum number of entries.
func New(size int) *Cache {
	lru, err := glru.New(size)
	if err != nil {
		panic(err)
	}
	return &Cache{lru: lru}
}

// Add inserts a new element to the cache
func (l *Cache) Add(key, value interface{}) {
	l.lru.Add(key, value)
}

// Get retrieves an element from the cache
func (l *Cache) Get(key interface{}) interface{} {
	value, ok := l.lru.Get(key)
	if !ok {
		return nil
	}
	return value
}
