package utils

import (
	"sync"
	"time"
)

// Cache stores memoized function call results.
//
// The zero Cache is ready for use.
type Cache struct {
	items sync.Map
}

type entry struct {
	sync.Mutex
	value         interface{}
	generated     time.Time
	lastTimeTaken time.Duration
}

func (c *Cache) getEntry(key string) *entry {
	e, _ := c.items.LoadOrStore(key, &entry{})
	return e.(*entry)
}

// Memoize calls and returns the results of the given function, or returns
// cached results from a previous call with the given key if the results are
// fresh enough. Only one call with a given key will be executed at a time.
// Function calls with a non-nil error are not cached.
func (c *Cache) Memoize(key string, minTTL time.Duration, fn func() (interface{}, error)) (interface{}, error) {
	e := c.getEntry(key)
	e.Lock()
	defer e.Unlock()

	if time.Since(e.generated) < (minTTL + e.lastTimeTaken) {
		// Cache hit!
		return e.value, nil
	}

	start := time.Now()
	v, err := fn()
	if err == nil {
		e.value = v
		e.generated = time.Now()
		e.lastTimeTaken = time.Since(start)
	}
	return v, err
}

// Delete removes the entry from the cache
func (c *Cache) Delete(key string) {
	c.items.Delete(key)
}

// ClearByAge clears the cache entries which age is longer than d
func (c *Cache) ClearByAge(d time.Duration) {
	c.items.Range(func(k interface{}, v interface{}) bool {
		v1, ok := c.items.Load(k)
		if ok {
			e := v1.(*entry)
			if time.Since(e.generated) > d {
				c.items.Delete(k)
			}
		}
		return true
	})
}
