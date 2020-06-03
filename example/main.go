package main

import (
	"fmt"
	"time"

	"github.com/xsean2020/gocache"
)

func main() {
	c := gocache.New(1*time.Second, time.Second)
	for i := 0; i < 10; i++ {
		time.Sleep(1 * time.Second)
		c.Add(fmt.Sprint(i), i)
	}
	select {}
}
