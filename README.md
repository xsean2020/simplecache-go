# Simple Go Mem Cache  With ttl and LRU 

### example
```
cache := gocache.New(1*time.Second, time.Second)
c.Add("key", 124)
```
