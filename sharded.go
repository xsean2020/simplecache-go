package simplecache

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	insecurerand "math/rand"
	"os"
	"runtime"
	"time"
)

// This is an experimental and unexported (for now) attempt at making a cache
// with better algorithmic complexity than the standard one, namely by
// preventing write locks of the entire cache when an item is added. As of the
// time of writing, the overhead of selecting buckets results in cache
// operations being about twice as slow as for the standard cache with small
// total cache sizes, and faster for larger ones.
//
// See cache_test.go for a few benchmarks.

type ShardedCache struct {
	*shardedCache
}

type shardedCache struct {
	seed uint32
	m    uint32
	cs   []*cache
	stop chan struct{}
}

// djb2 with better shuffling. 5x faster than FNV with the hash.Hash overhead.
func djb33(seed uint32, k string) uint32 {
	var (
		l = uint32(len(k))
		d = 5381 + seed + l
		i = uint32(0)
	)
	// Why is all this 5x faster than a for loop?
	if l >= 4 {
		for i < l-4 {
			d = (d * 33) ^ uint32(k[i])
			d = (d * 33) ^ uint32(k[i+1])
			d = (d * 33) ^ uint32(k[i+2])
			d = (d * 33) ^ uint32(k[i+3])
			i += 4
		}
	}
	switch l - i {
	case 1:
	case 2:
		d = (d * 33) ^ uint32(k[i])
	case 3:
		d = (d * 33) ^ uint32(k[i])
		d = (d * 33) ^ uint32(k[i+1])
	case 4:
		d = (d * 33) ^ uint32(k[i])
		d = (d * 33) ^ uint32(k[i+1])
		d = (d * 33) ^ uint32(k[i+2])
	}
	return d ^ (d >> 16)
}

func (sc *shardedCache) bucket(k K) *cache {
	return sc.cs[djb33(sc.seed, fmt.Sprint(k))%sc.m]
}

func (sc *shardedCache) AddTTL(k K, x interface{}, d time.Duration) {
	sc.bucket(k).AddWithTTL(k, x, d)
}

func (sc *shardedCache) Add(k K, x interface{}) {
	sc.bucket(k).Add(k, x)
}

func (sc *shardedCache) Get(k K) (interface{}, bool) {
	return sc.bucket(k).Get(k)
}

func (sc *shardedCache) Remove(k string) {
	sc.bucket(k).Remove(k)
}

func (sc *shardedCache) Tidy() {
	for _, v := range sc.cs {
		v.Tidy()
	}
}

// Returns the items in the cache. This may include items that have expired,
// but have not yet been cleaned up. If this is significant, the Expiration
// fields of the items should be checked. Note that explicit synchronization
// is needed to use a cache and its corresponding Items() return values at
// the same time, as the maps are shared.
func (sc *shardedCache) Keys() []interface{} {
	var ks []interface{}
	for _, v := range sc.cs {
		ks = append(ks, v.Keys()...)
	}
	return ks
}

func (sc *shardedCache) Purge() {
	for _, v := range sc.cs {
		v.Purge()
	}
}

func (sc *shardedCache) Foreach(fn func(k, v interface{})) {
	for _, v := range sc.cs {
		v.Foreach(fn)
	}
}

func newShardedCache(n int, de time.Duration) *shardedCache {
	max := big.NewInt(0).SetUint64(uint64(math.MaxUint32))
	rnd, err := rand.Int(rand.Reader, max)
	var seed uint32
	if err != nil {
		os.Stderr.Write([]byte("WARNING: go-cache's newShardedCache failed to read from the system CSPRNG (/dev/urandom or equivalent.) Your system's security may be compromised. Continuing with an insecure seed.\n"))
		seed = insecurerand.Uint32()
	} else {
		seed = uint32(rnd.Uint64())
	}
	sc := &shardedCache{
		seed: seed,
		m:    uint32(n),
		cs:   make([]*cache, n),
		stop: make(chan struct{}),
	}
	for i := 0; i < n; i++ {
		c := &cache{
			defaultExpiration: de,
			indices:           map[K]int{},
		}
		sc.cs[i] = c
	}
	return sc
}

func (sc *shardedCache) run(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			for _, c := range sc.cs {
				c.Tidy()
			}
		case <-sc.stop:
			ticker.Stop()
			return
		}
	}
}

func NewSharded(defaultExpiration, cleanupInterval time.Duration, shards int) *ShardedCache {
	if defaultExpiration == 0 {
		defaultExpiration = -1
	}
	sc := newShardedCache(shards, defaultExpiration)
	SC := &ShardedCache{sc}
	if cleanupInterval > 0 {
		go sc.run(cleanupInterval)
		runtime.SetFinalizer(SC, func(sc *ShardedCache) {
			close(sc.stop)
		})
	}
	return SC
}
