package cache

import (
	"sync"
	"time"

	"staccato/pkg/models"
)

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Value      interface{}
	Expiration time.Time
}

// IsExpired checks if the cache entry has expired
func (e *CacheEntry) IsExpired() bool {
	return time.Now().After(e.Expiration)
}

// MemoryCache implements a simple in-memory cache
type MemoryCache struct {
	items map[string]*CacheEntry
	mutex sync.RWMutex
	ttl   time.Duration
}

// NewMemoryCache creates a new memory cache
func NewMemoryCache(ttl time.Duration) *MemoryCache {
	cache := &MemoryCache{
		items: make(map[string]*CacheEntry),
		mutex: sync.RWMutex{},
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go cache.cleanupExpired()

	return cache
}

// Set stores a value in the cache
func (c *MemoryCache) Set(key string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items[key] = &CacheEntry{
		Value:      value,
		Expiration: time.Now().Add(c.ttl),
	}
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	entry, exists := c.items[key]
	if !exists || entry.IsExpired() {
		return nil, false
	}

	return entry.Value, true
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *MemoryCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.items = make(map[string]*CacheEntry)
}

// Size returns the number of items in the cache
func (c *MemoryCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return len(c.items)
}

// cleanupExpired removes expired entries periodically
func (c *MemoryCache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute * 5) // Cleanup every 5 minutes
	defer ticker.Stop()

	for range ticker.C {
		c.mutex.Lock()
		for key, entry := range c.items {
			if entry.IsExpired() {
				delete(c.items, key)
			}
		}
		c.mutex.Unlock()
	}
}

// TrackCache provides convenience methods for caching tracks
type TrackCache struct {
	*MemoryCache
}

// NewTrackCache creates a new track cache
func NewTrackCache() *TrackCache {
	return &TrackCache{
		MemoryCache: NewMemoryCache(15 * time.Minute), // Cache tracks for 15 minutes
	}
}

// SetTracks caches a slice of tracks
func (tc *TrackCache) SetTracks(key string, tracks []models.Track) {
	tc.Set(key, tracks)
}

// GetTracks retrieves cached tracks
func (tc *TrackCache) GetTracks(key string) ([]models.Track, bool) {
	value, exists := tc.Get(key)
	if !exists {
		return nil, false
	}

	tracks, ok := value.([]models.Track)
	return tracks, ok
}

// SetTrack caches a single track
func (tc *TrackCache) SetTrack(key string, track *models.Track) {
	tc.Set(key, track)
}

// GetTrack retrieves a cached track
func (tc *TrackCache) GetTrack(key string) (*models.Track, bool) {
	value, exists := tc.Get(key)
	if !exists {
		return nil, false
	}

	track, ok := value.(*models.Track)
	return track, ok
}
