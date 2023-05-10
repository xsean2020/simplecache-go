package simplecache

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

type entry[K comparable, V any] struct {
	Expiration int64
	sync.Mutex
	key   K
	value V
}

func (e *entry[K, V]) Expired() bool {
	if e.Expiration == 0 {
		return false
	}
	return time.Now().UnixNano() > e.Expiration
}

const (
	// For use with functions that take an expiration time.
	NoExpiration time.Duration = -1
	// For use with functions that take an expiration time. Equivalent to
	// passing in the same expiration duration as was given to New() or
	// NewFrom() when the cache was created (e.g. 5 minutes.)
	DefaultExpiration time.Duration = 0
)

type Cache[K comparable, V any] struct {
	*cache[K, V]
	// If this is confusing, see the comment at the bottom of New()
}

type cache[K comparable, V any] struct {
	sync.RWMutex
	defaultExpiration time.Duration
	items             []entry[K, V]
	indices           map[K]int
	onEvicted         func(K, V)
	stop              chan struct{}
}

// Add an item to the cache, replacing any existing item. If the duration is 0
// (DefaultExpiration), the cache's default expiration time is used. If it is -1
// (NoExpiration), the item never expires.
func (c *cache[K, V]) Set(k K, x V, d time.Duration) {
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
		c.items = append(c.items, entry[K, V]{key: k, value: x, Expiration: e})
		c.indices[k] = idx
	}
	// TODO: Calls to mu.Unlock are currently not deferred because defer
	// adds ~200 ns (as of go1.)
	c.Unlock()
}

func (c *cache[K, V]) SetDefault(k K, v V) {
	c.Set(k, v, DefaultExpiration)
}

func (c *cache[K, V]) set(k K, x V, d time.Duration) {
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
		c.items = append(c.items, entry[K, V]{key: k, value: x, Expiration: e})
		c.indices[k] = idx
	}
}

// Add an item to the cache, replacing any existing item, using the default
// expiration.
func (c *cache[K, V]) Add(k K, x V, d time.Duration) error {
	c.Lock()
	_, found := c.get(k)
	if found {
		c.Unlock()
		return fmt.Errorf("Item %v alread exists ", k)
	}
	c.set(k, x, d)
	c.Unlock()
	return nil
}

// Checks if a key exists in cache
func (c *cache[K, V]) Contains(k K) bool {
	c.RLock()
	_, found := c.indices[k]
	c.RUnlock()
	return found
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache[K, V]) get(k K) (v V, ok bool) {
	// "Inlining" of get and Expired
	idx, found := c.indices[k]
	if !found {
		return v, false
	}

	item := &c.items[idx]
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			return v, false
		}
	}
	return item.value, true
}

// Get an item from the cache. Returns the item or nil, and a bool indicating
// whether the key was found.
func (c *cache[K, V]) Get(k K) (v V, ok bool) {
	c.RLock()
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return v, false
	}

	if c.items[idx].Expiration > 0 {
		if time.Now().UnixNano() > c.items[idx].Expiration {
			c.RUnlock()
			return v, false
		}
	}
	v = c.items[idx].value
	c.RUnlock()
	return v, true
}

// Get renewal when lt defaltExpiration/2
func (c *cache[K, V]) GetAndRenewal(k K) (v V, ok bool) {
	c.RLock()
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return v, false
	}

	c.items[idx].Lock()
	now := time.Now().UnixNano()
	exp := int64(c.defaultExpiration / 3)
	if c.items[idx].Expiration > 0 && c.items[idx].Expiration-now <= exp {
		c.items[idx].Expiration += exp
	}
	c.items[idx].Unlock()
	v = c.items[idx].value

	c.RUnlock()
	return v, true
}

// GetPointer
func (c *cache[K, V]) GetPointer(k K) (v *V, ok bool) {
	c.RLock()
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return nil, false
	}

	if c.items[idx].Expiration > 0 {
		if time.Now().UnixNano() > c.items[idx].Expiration {
			c.RUnlock()
			return v, false
		}
	}
	v = &c.items[idx].value
	c.RUnlock()
	return v, true
}

// GetWithExpiration returns an item and its expiration time from the cache.
// It returns the item or nil, the expiration time if one is set (if the item
// never expires a zero value for time.Time is returned), and a bool indicating
// whether the key was found.

func (c *cache[K, V]) GetWithExpiration(k K) (v V, t time.Time, ok bool) {
	c.RLock()
	idx, found := c.indices[k]
	if !found {
		c.RUnlock()
		return v, t, false
	}

	item := &c.items[idx]
	if item.Expiration > 0 {
		if time.Now().UnixNano() > item.Expiration {
			c.RUnlock()
			return v, t, false
		}

		// Return the item and the expiration time
		c.RUnlock()
		return item.value, time.Unix(0, item.Expiration), true
	}

	// If expiration <= 0 (i.e. no expiration time set) then return the item
	// and a zeroed time.Time
	c.RUnlock()
	return item.value, t, true
}

// Delete an item from the cache. Does nothing if the key is not in the cache.
func (c *cache[K, V]) Delete(k K) {
	c.Lock()
	v, evicted := c.delete(k)
	c.Unlock()
	if evicted {
		c.onEvicted(k, v)
	}
}

func (c *cache[K, V]) delete(k K) (v V, ok bool) {
	idx, found := c.indices[k]
	if !found {
		return
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
	return v, false
}

// Delete all expired items from the cache.
func (c *cache[K, V]) DeleteExpired() {
	var ks []K
	var vs []V
	now := time.Now().UnixNano()
	c.Lock()
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
	c.Unlock()
	for i := range ks {
		c.onEvicted(ks[i], vs[i])
	}
}

// Sets an (optional) function that is called with the key and value when an
// item is evicted from the cache. (Including when it is deleted manually, but
// not when it is overwritten.) Set to nil to disable.
func (c *cache[K, V]) OnEvicted(f func(K, V)) {
	c.Lock()
	c.onEvicted = f
	c.Unlock()
}

// Copies all unexpired items in the cache into a new map and returns it.
func (c *cache[K, V]) Keys() []K {
	var ks []K
	c.RLock()
	defer c.RUnlock()
	now := time.Now().UnixNano()
	for _, v := range c.items {
		// "Inlining" of Expired
		if v.Expiration > 0 {
			if now > v.Expiration {
				continue
			}
		}
		ks = append(ks, v.key)
	}
	return ks
}

// Returns the number of items in the cache. This may include items that have
// expired, but have not yet been cleaned up.
func (c *cache[K, V]) Len() int {
	c.RLock()
	n := len(c.items)
	c.RUnlock()
	return n
}

// Vist all items from the cache.
func (c *cache[K, V]) Foreach(fn func(k K, v V)) {
	c.Lock()
	for i := range c.items {
		fn(c.items[i].key, c.items[i].value)
	}
	c.Unlock()
}

// Delete all items from the cache.
func (c *cache[K, V]) Purge() {
	c.Lock()
	var zero entry[K, V]
	for i := range c.items {
		c.items[i] = zero // 清空数据
	}
	c.items = c.items[:0]
	c.indices = make(map[K]int)
	c.Unlock()
}

func (c *cache[K, V]) run(interval time.Duration) {
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

func newCache[K comparable, V any](initcap int, de time.Duration) *cache[K, V] {
	if de == 0 {
		de = -1
	}
	c := &cache[K, V]{
		defaultExpiration: de,
		items:             make([]entry[K, V], 0, initcap),
		indices:           make(map[K]int),
		stop:              make(chan struct{}),
	}
	return c
}

func newCacheWithJanitor[K comparable, V any](initcap int, de time.Duration, ci time.Duration) *Cache[K, V] {
	c := newCache[K, V](initcap, de)
	C := &Cache[K, V]{c}
	if ci > 0 {
		go c.run(ci)
		runtime.SetFinalizer(C, func(C *Cache[K, V]) {
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
func New[K comparable, V any](initcap int, defaultExpiration, cleanupInterval time.Duration) *Cache[K, V] {
	return newCacheWithJanitor[K, V](initcap, defaultExpiration, cleanupInterval)
}
