package gocache

import (
	cheap "container/heap"
	"sync"
	"time"
)

type GCache struct {
	h               *heap
	recycleInterval time.Duration
	maxLifeTime     time.Duration
	sync.RWMutex
}

func (c *GCache) Add(key string, x interface{}) {
	c.Lock()
	defer c.Unlock()
	cheap.Push(c.h, &entry{key: key, data: x, expired: time.Now().Add(c.maxLifeTime)})

}

func (c *GCache) Get(key string) Entry {
	c.RLock()
	defer c.RUnlock()
	return c.h.get(key)
}

func (c *GCache) Len() int {
	c.RLock()
	defer c.RUnlock()
	return c.h.Len()
}

func (c *GCache) Pop() Entry {
	c.Lock()
	defer c.Unlock()
	return cheap.Pop(c.h).(*entry)
}

func (c *GCache) Remove(x string) {
	c.Lock()
	defer c.Unlock()
	c.h.remove(x)
}

func (c *GCache) Top() Entry {
	c.RLock()
	defer c.RUnlock()
	if c.h.Len() <= 0 {
		return nil
	}
	return c.h.top().(*entry)
}

func New(interval, maxLifeTime time.Duration) *GCache {
	o := &GCache{
		recycleInterval: interval,
		maxLifeTime:     maxLifeTime,
		h: &heap{
			indices: make(map[string]int),
		},
	}
	go o.gc()
	return o
}

func (c *GCache) gc() {
	ticker := time.NewTicker(c.recycleInterval)
	for {
		select {
		case now := <-ticker.C:
			for {
				top, _ := c.Top().(*entry)
				if top == nil || top.expired.After(now) {
					break
				}
				c.Remove(top.key)
			}
		}
	}
}
