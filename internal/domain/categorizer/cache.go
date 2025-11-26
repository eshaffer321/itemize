package categorizer

import (
	"sync"
)

// MemoryCache is a simple in-memory cache implementation
type MemoryCache struct {
	mu    sync.RWMutex
	store map[string]string
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		store: make(map[string]string),
	}
}

// Get retrieves a value from cache
func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, found := c.store[key]
	return value, found
}

// Set stores a value in cache
func (c *MemoryCache) Set(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store[key] = value
}

// Clear removes all entries from cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.store = make(map[string]string)
}

// Size returns the number of cached entries
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.store)
}
