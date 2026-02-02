package cache

import (
	"testing"
)

func TestNewMemoryCache(t *testing.T) {
	c := NewMemoryCache()
	if c == nil {
		t.Fatal("NewMemoryCache returned nil")
	}
	if c.Size() != 0 {
		t.Errorf("expected empty cache, got size %d", c.Size())
	}
}

func TestMemoryCache_PutAndGet(t *testing.T) {
	c := NewMemoryCache()

	// Test Put and Get
	c.Put("key1", "value1")

	value, found := c.Get("key1")
	if !found {
		t.Error("expected to find key1")
	}
	if value != "value1" {
		t.Errorf("expected value1, got %s", value)
	}

	// Test Size
	if c.Size() != 1 {
		t.Errorf("expected size 1, got %d", c.Size())
	}
}

func TestMemoryCache_Get_NotFound(t *testing.T) {
	c := NewMemoryCache()

	value, found := c.Get("nonexistent")
	if found {
		t.Error("expected not to find nonexistent key")
	}
	if value != "" {
		t.Errorf("expected empty string for not found, got %s", value)
	}
}

func TestMemoryCache_Put_Overwrite(t *testing.T) {
	c := NewMemoryCache()

	c.Put("key1", "value1")
	c.Put("key1", "value2") // Overwrite

	value, found := c.Get("key1")
	if !found {
		t.Error("expected to find key1")
	}
	if value != "value2" {
		t.Errorf("expected value2 after overwrite, got %s", value)
	}

	if c.Size() != 1 {
		t.Errorf("expected size 1 after overwrite, got %d", c.Size())
	}
}

func TestMemoryCache_Clear(t *testing.T) {
	c := NewMemoryCache()

	c.Put("key1", "value1")
	c.Put("key2", "value2")

	if c.Size() != 2 {
		t.Errorf("expected size 2, got %d", c.Size())
	}

	c.Clear()

	if c.Size() != 0 {
		t.Errorf("expected size 0 after clear, got %d", c.Size())
	}

	_, found := c.Get("key1")
	if found {
		t.Error("expected key1 to be cleared")
	}
}

func TestMemoryCache_ConcurrentAccess(t *testing.T) {
	c := NewMemoryCache()

	// Run concurrent writes
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				c.Put("key", "value")
			}
			done <- true
		}(i)
	}

	// Run concurrent reads
	for i := 0; i < 10; i++ {
		go func(n int) {
			for j := 0; j < 100; j++ {
				c.Get("key")
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 20; i++ {
		<-done
	}

	// Cache should still be in a valid state
	value, found := c.Get("key")
	if !found {
		t.Error("expected to find key after concurrent access")
	}
	if value != "value" {
		t.Errorf("expected value, got %s", value)
	}
}

func TestMemoryCache_EmptyKey(t *testing.T) {
	c := NewMemoryCache()

	c.Put("", "empty-key-value")

	value, found := c.Get("")
	if !found {
		t.Error("expected to find empty key")
	}
	if value != "empty-key-value" {
		t.Errorf("expected empty-key-value, got %s", value)
	}
}

func TestMemoryCache_EmptyValue(t *testing.T) {
	c := NewMemoryCache()

	c.Put("empty-value-key", "")

	value, found := c.Get("empty-value-key")
	if !found {
		t.Error("expected to find key with empty value")
	}
	if value != "" {
		t.Errorf("expected empty string, got %s", value)
	}
}

func TestMemoryCache_LargeValues(t *testing.T) {
	c := NewMemoryCache()

	largeValue := make([]byte, 10000)
	for i := range largeValue {
		largeValue[i] = 'a'
	}
	value := string(largeValue)

	c.Put("large", value)

	retrieved, found := c.Get("large")
	if !found {
		t.Error("expected to find large value")
	}
	if retrieved != value {
		t.Error("retrieved value does not match original")
	}
}
