package cache

import (
	"sync"
	"time"
)

type Cache interface {
	Get(key string) (interface{}, bool, error)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Clear() error
}

type cacheEntry struct {
	value      interface{}
	expiration time.Time
}

type InMemoryCache struct {
	mu      sync.RWMutex
	items   map[string]cacheEntry
	cleanup time.Duration
}

func NewInMemoryCache(cleanupInterval time.Duration) *InMemoryCache {
	c := &InMemoryCache{
		items:   make(map[string]cacheEntry),
		cleanup: cleanupInterval,
	}
	go c.startEvictionLoop()
	return c
}

func (c *InMemoryCache) Get(key string) (interface{}, bool, error) {
	c.mu.RLock()
	entry, exists := c.items[key]
	c.mu.RUnlock()

	if !exists {
		return nil, false, nil
	}

	if !entry.expiration.IsZero() && time.Now().After(entry.expiration) {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil, false, nil
	}

	return entry.value, true, nil
}

func (c *InMemoryCache) Set(key string, value interface{}, ttl time.Duration) error {
	var expiration time.Time
	if ttl > 0 {
		expiration = time.Now().Add(ttl)
	}

	c.mu.Lock()
	c.items[key] = cacheEntry{
		value:      value,
		expiration: expiration,
	}
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) Delete(key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) Clear() error {
	c.mu.Lock()
	c.items = make(map[string]cacheEntry)
	c.mu.Unlock()
	return nil
}

func (c *InMemoryCache) EvictExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for k, v := range c.items {
		if !v.expiration.IsZero() && now.After(v.expiration) {
			delete(c.items, k)
		}
	}
}

func (c *InMemoryCache) startEvictionLoop() {
	ticker := time.NewTicker(c.cleanup)
	for range ticker.C {
		c.EvictExpired()
	}
}
