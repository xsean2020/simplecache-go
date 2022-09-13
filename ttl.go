package simplecache

import (
	"runtime"
	"sync"
	"time"
)

type K = interface{}

type entry struct {
	value      interface{}
	key        K
	Expiration int64
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache struct {
	*cache
	// If this is confusing, see the comment at the bottom of New()
}

type cache struct {
	sync.RWMutex
	defaultExpiration time.Duration
	items             []entry
	indices           map[K]int
	onEvicted         func(interface{}, interface{})
	stop              chan struct{}
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache) AddTTL(k K, x interface{}, d time.Duration) {
	// "Inlining" of set
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}
	c.Lock()
	if idx, ok := c.indices[k]; ok {
		c.items[idx].value = x
		c.items[idx].key = k
		c.items[idx].Expiration = e
	} else {
		idx := len(c.items)
		c.items = append(c.items, entry{key: k, value: x, Expiration: e})
		c.indices[k] = idx
	}
	// TODO: Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.Unlock()
}

func (c *cache) setTTL(k K, x interface{}, d time.Duration) {
	var e int64
	if d == DefaultExpiration {
		d = c.defaultExpiration
	}
	if d > 0 {
		e = time.Now().Add(d).UnixNano()
	}

	if idx, ok := c.indices[k]; ok {
		c.items[idx].value = x
		c.items[idx].key = k
		c.items[idx].Expiration = e
	} else {
		idx := len(c.items)
		c.items = append(c.items, entry{key: k, value: x, Expiration: e})
		c.indices[k] = idx
	}
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *cache) Add(k string, x interface{}) {
	c.Lock()
	c.setTTL(k, x, DefaultExpiration)
	c.Unlock()
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache) Get(k K) (interface{}, bool) {
	c.RLock()
	// "Inlining" of get and Expired
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return nil, false
	}

	item := &c.items[idx]
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.RUnlock()
			return nil, false
		}
	}
	c.RUnlock()
	return item.value, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.
func (c *cache) GetWithExpiration(k K) (interface{}, time.Time, bool) {
	c.RLock()
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return nil, time.Time{}, false
	}

	item := &c.items[idx]
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.RUnlock()
			return nil, time.Time{}, false
		}

		// Return the item and the expiration time
		c.RUnlock()
		return item.value, time.Unix(0, item.Expiration), true
	}

	// If expiration <= 0 (i.e. no expiration time set) then return the item
	// and a zeroed time.Time
	c.RUnlock()
	return item.value, time.Time{}, true
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache) Delete(k string) {
	c.Lock()
	v, evicted := c.delete(k)
	c.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache) delete(k K) (interface{}, bool) {
	idx, found := c.indices[k]
	if !found {
		return nil, false
	}
	n := len(c.indices) - 1
	c.items[n], c.items[idx] = c.items[idx], c.items[n]
	c.indices[c.items[idx].key] = idx
	delete(c.indices, k)
	if c.onEvicted != nil {
		x := c.items[n]
		c.items = c.items[:n]
		return x.value, true
	}
	c.items = c.items[:n]
	return nil, false
}

// Delete all expired items from the cache.
func (c *cache) DeleteExpired() {
	var ks []interface{}
	var vs []interface{}

	now := time.Now().UnixNano()
	c.RLock()
	for _, v := range c.items {
		// "Inlining" of expired
		if v.Expiration > 0 && now > v.Expiration {
			_, evicted := c.delete(v.key)
			if evicted {
				ks = append(ks, v.key)
				vs = append(vs, v.value)
			}
		}
	}
	c.RUnlock()
	for i := range ks {
		c.onEvicted(ks[i], vs[i])
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *cache) OnEvicted(f func(interface{}, interface{})) {
	c.Lock()
	c.onEvicted = f
	c.Unlock()
}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *cache) Keys() []interface{} {
	var ks []interface{}
	c.RLock()
	defer c.RUnlock()
	now := time.Now().UnixNano()
	for k, v := range c.items {
		// "Inlining" of Expired
		if v.Expiration > 0 {
			if now > v.Expiration {
				continue
			}
		}
		// m[k] = v
		ks = append(ks, k)
	}
	return ks
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *cache) Count() int {
	c.RLock()
	n := len(c.items)
	c.RUnlock()
	return n
}

// Vist all items from the cache.
func (c *cache) Foreach(fn func(k, v interface{})) {
	c.Lock()
	for i := range c.items {
		fn(c.items[i].key, c.items[i].value)
	}
	c.Unlock()
}

// Delete all items from the cache.
func (c *cache) Purge() {
	c.Lock()
	c.items = c.items[:0]
	c.indices = make(map[K]int)
	c.Unlock()
}

func (c *cache) run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-c.stop:
			ticker.Stop()
			return
		}
	}
}

func newCache(de time.Duration) *cache {
	if de == 0 {
		de = -1
	}
	c := &cache{
		defaultExpiration: de,
		indices:           make(map[K]int),
		stop:              make(chan struct{}),
	}
	return c
}

func newCacheWithJanitor(de time.Duration, ci time.Duration) *Cache {
	c := newCache(de)
	C := &Cache{c}
	if ci > 0 {
		go c.run(ci)
		runtime.SetFinalizer(C, func(C *Cache) {
			close(C.cache.stop)
		})
	}
	return C
}

// Return a new cache with a given default expiration duration and cleanup
// interval. If the expiration duration is less than one (or NoExpiration),
// the items in the cache never expire (by default), and must be deleted
// manually. If the cleanup interval is less than one, expired items are not
// deleted from the cache before calling c.DeleteExpired().
func New(defaultExpiration, cleanupInterval time.Duration) *Cache {
	return newCacheWithJanitor(defaultExpiration, cleanupInterval)
}
