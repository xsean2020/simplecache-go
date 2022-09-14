# simplecache

clone from https://github.com/patrickmn/go-cache.git

simplecache is an in-memory key:value store/cache 

Any object can be stored, for a given duration or forever, and the cache can be
safely used by multiple goroutines.

### Installation

`go get github.com/xsean2020/simplecache-go`

### Usage

```go
import (
	"fmt"
	"github.com/xsean2020/simplecache-go"
	"time"
)

func main() {
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
	c := simplecache.New(5*time.Minute, 10*time.Minute)

	// Set the value of the key "foo" to "bar", with the default expiration time
	c.AddWithTTL("foo", "bar", cache.DefaultExpiration)

        // Set value with default ttl 
	c.Add("baz", 42)

	// Get the string associated with the key "foo" from the cache
	foo, found := c.Get("foo")
	if found {
		fmt.Println(foo)
	}

	foo, found := c.Get("foo")
	if found {
		MyFunction(foo.(string))
	}

       // Or instead of follow	
	if x, found := c.Get("foo"); found {
		foo := x.(string)
		// ...
	}

	// or
	var foo string
	if x, found := c.Get("foo"); found {
		foo = x.(string)
	}
	// ...
	// foo can then be passed around freely as a string

	// Want performance? Store pointers!
	c.AddWithTTL("foo", &MyStruct, cache.DefaultExpiration)
	if x, found := c.Get("foo"); found {
		foo := x.(*MyStruct)
			// ...
	}
}
```


### Generic  usage

```go 
import (
	"fmt"
	simplecache "github.com/xsean2020/simplecache-go/generic"
	"time"
)

func main() {
	// Create a cache with a default expiration time of 5 minutes, and which
	// purges expired items every 10 minutes
       // Key must be string , value is any
	c := simplecache.New[string,any](5*time.Minute, 10*time.Minute)

	// Set the value of the key "foo" to "bar", with the default expiration time
	c.AddWithTTL("foo", "bar", cache.DefaultExpiration)

        // Set value with default ttl 
	c.Add("baz", 42)

	// Get the string associated with the key "foo" from the cache
	foo, found := c.Get("foo")
	if found {
		fmt.Println(foo)
	}

	foo, found := c.Get("foo")
	if found {
		MyFunction(foo.(string))
	}

       // Or instead of follow	
	if x, found := c.Get("foo"); found {
		foo := x.(string)
		// ...
	}

	// or
	var foo string
	if x, found := c.Get("foo"); found {
		foo = x.(string)
	}
	// ...
	// foo can then be passed around freely as a string
       // or
       c1 :=  simplecache.New[string,string](5*time.Minute, 10*time.Minute)
       c1.AddWithTTL("foo", "foovalue", cache.DefaultExpiration)
       x, found := c2.Get("foo") // x is string type


	// Want performance? Store pointers!
	c.AddWithTTL("foo", &MyStruct, cache.DefaultExpiration)
	if x, found := c.Get("foo"); found {
		foo := x.(*MyStruct)
	}

        //  or 
       c2 :=  simplecache.New[string,*MyStruct](5*time.Minute, 10*time.Minute)
       c2.AddWithTTL("foo", &MyStruct, cache.DefaultExpiration)
       x, found := c2.Get("foo") // X is *Mystruct type







}
```
