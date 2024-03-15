package cache

import (
	"errors"
	"sync"
	"time"
)

// Cache stores arbitrary data with expiration time.
type Cache struct {
	items sync.Map
	close chan struct{}

	returnStale       bool
	janitorInterval   time.Duration
	defaultExpiration time.Duration
}

// An item represents arbitrary data with expiration time.
type item struct {
	data    any
	expires int64
}

type option func(*Cache) error

func WithJanitor(interval time.Duration) option {
	return func(c *Cache) error {
		if interval <= 0 {
			return errors.New("janitor interval must be greater than 0")
		}
		c.janitorInterval = interval
		c.close = make(chan struct{})
		return nil
	}
}

func WithGetReturnStale() option {
	return func(c *Cache) error {
		c.returnStale = true
		return nil
	}
}

func WithDefaultExpiration(expiration time.Duration) option {
	return func(c *Cache) error {
		c.defaultExpiration = expiration
		return nil
	}
}

func New(opts ...option) *Cache {
	cache := &Cache{
		janitorInterval:   -1,
		defaultExpiration: -1,
	}

	for _, opt := range opts {
		opt(cache)
	}

	if cache.janitorInterval > 0 {
		go cache.janitor()
	}

	return cache
}

func (c *Cache) janitor() {
	ticker := time.NewTicker(c.janitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now().UnixNano()

			c.items.Range(func(key, value any) bool {
				item := value.(item)

				if item.expires > 0 && now > item.expires {
					c.items.Delete(key)
				}

				return true
			})

		case <-c.close:
			return
		}
	}
}

// Get gets the value for the given key.
func (cache *Cache) Get(key any) (any, bool) {
	obj, exists := cache.items.Load(key)

	if !exists {
		return nil, false
	}

	item := obj.(item)

	if item.expires > 0 && time.Now().UnixNano() > item.expires {
		if cache.returnStale {
			return item.data, false
		}

		return nil, false
	}

	return item.data, true
}

// Set sets a value for the given key with the specified expiration duration.
// If the duration is less than 0, the value never expires.
func (cache *Cache) Set(key any, value any, duration time.Duration) {
	var expires int64

	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}

	cache.items.Store(key, item{
		data:    value,
		expires: expires,
	})
}

// SetDefault sets a value for the given key with the default expiration duration.
func (cache *Cache) SetDefault(key any, value any) {
	cache.Set(key, value, cache.defaultExpiration)
}

// Range calls f sequentially for each key and value present in the cache.
// If f returns false, range stops the iteration.
func (cache *Cache) Range(f func(key, value any) bool) {
	now := time.Now().UnixNano()

	fn := func(key, value any) bool {
		item := value.(item)

		if item.expires > 0 && now > item.expires {
			return true
		}

		return f(key, item.data)
	}

	cache.items.Range(fn)
}

// Delete deletes the key and its value from the cache.
func (cache *Cache) Delete(key any) {
	cache.items.Delete(key)
}

// Close closes the cache and frees up resources.
func (cache *Cache) Close() {
	cache.close <- struct{}{}
	cache.items = sync.Map{}
}
