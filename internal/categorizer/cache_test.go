package categorizer

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryCache_GetSet(t *testing.T) {
	cache := NewMemoryCache()

	// Test Set and Get
	cache.Set("milk", "cat_1")
	
	value, found := cache.Get("milk")
	assert.True(t, found)
	assert.Equal(t, "cat_1", value)

	// Test Get non-existent
	value, found = cache.Get("bread")
	assert.False(t, found)
	assert.Empty(t, value)
}

func TestMemoryCache_Clear(t *testing.T) {
	cache := NewMemoryCache()

	// Add items
	cache.Set("milk", "cat_1")
	cache.Set("bread", "cat_1")
	
	assert.Equal(t, 2, cache.Size())

	// Clear cache
	cache.Clear()
	
	assert.Equal(t, 0, cache.Size())
	
	// Verify items are gone
	_, found := cache.Get("milk")
	assert.False(t, found)
}

func TestMemoryCache_Concurrent(t *testing.T) {
	cache := NewMemoryCache()
	
	// Test concurrent access
	var wg sync.WaitGroup
	numGoroutines := 100
	
	wg.Add(numGoroutines * 2)
	
	// Writers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id)
			value := fmt.Sprintf("value_%d", id)
			cache.Set(key, value)
		}(i)
	}
	
	// Readers
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key_%d", id)
			// May or may not find the value depending on timing
			cache.Get(key)
		}(i)
	}
	
	wg.Wait()
	
	// Verify at least some items were cached
	assert.Greater(t, cache.Size(), 0)
}

func TestMemoryCache_Size(t *testing.T) {
	cache := NewMemoryCache()
	
	assert.Equal(t, 0, cache.Size())
	
	cache.Set("item1", "cat1")
	assert.Equal(t, 1, cache.Size())
	
	cache.Set("item2", "cat2")
	assert.Equal(t, 2, cache.Size())
	
	// Overwrite existing
	cache.Set("item1", "cat3")
	assert.Equal(t, 2, cache.Size())
}

func TestMemoryCache_Integration(t *testing.T) {
	cache := NewMemoryCache()
	
	// Simulate categorization caching
	items := []string{
		"great value milk",
		"bounty paper towels",
		"iphone charger",
	}
	
	categories := []string{
		"cat_groceries",
		"cat_household",
		"cat_electronics",
	}
	
	// Cache mappings
	cache.Set(items[0], categories[0])
	cache.Set(items[1], categories[1])
	cache.Set(items[2], categories[2])
	
	// Verify all cached correctly
	for i, item := range items {
		value, found := cache.Get(item)
		require.True(t, found, "Item %s should be cached", item)
		assert.Equal(t, categories[i], value)
	}
}