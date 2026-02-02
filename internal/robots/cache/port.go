package cache

// Cache defines the port interface for robots.txt result caching.
// This interface follows the port-adapter pattern, allowing different
// cache implementations to be swapped without changing the fetcher logic.
//
// The cache uses simple key-value storage (strings only) to ensure
// flexibility and avoid tight coupling to specific data structures.
// Implementations are responsible for serialization/deserialization.
type Cache interface {
	// Get retrieves a value from the cache by key.
	// Returns the cached value and true if found, or empty string and false if not found.
	// This method is read-only and should not modify cache state.
	Get(key string) (string, bool)

	// Put stores a key-value pair in the cache.
	// If the key already exists, the value is overwritten.
	// The cache lives only for the duration of the crawling session (no persistence).
	Put(key string, value string)
}
