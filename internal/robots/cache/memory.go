package cache

import "sync"

// MemoryCache is an in-memory implementation of the Cache interface.
// It uses a map for storage and provides thread-safe operations via RWMutex.
//
// This adapter stores values as simple strings (key-value pairs) without
// any persistence. The cache lives only for the duration of the crawling session.
//
// Note: This implementation does not include hit/miss tracking or expiration logic.
// Those features can be added in future iterations if needed.
type MemoryCache struct {
	mu   sync.RWMutex
	data map[string]string
}

// NewMemoryCache creates a new in-memory cache instance.
// The cache is initialized empty and ready for use.
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		data: make(map[string]string),
	}
}

// Get retrieves a value from the cache by key.
// This method is thread-safe for concurrent reads.
// Returns the cached value and true if the key exists,
// or empty string and false if the key is not found.
func (c *MemoryCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	value, exists := c.data[key]
	return value, exists
}

// Put stores a key-value pair in the cache.
// This method is thread-safe for concurrent writes.
// If the key already exists, the value is overwritten.
// Both key and value are stored as plain strings.
func (c *MemoryCache) Put(key string, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = value
}

// Clear removes all entries from the cache.
// This method is primarily useful for testing.
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data = make(map[string]string)
}

// Size returns the number of entries in the cache.
// This method is primarily useful for testing and diagnostics.
func (c *MemoryCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.data)
}
