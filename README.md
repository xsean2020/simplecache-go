# A go mem cache base on heap

### example
`
cache := gocache.New(1*time.Second, time.Second)
c.Add("key", 124)

`
