package hypatia

import (
	"strconv"
	"sync"
	"testing"
)

func TestCache(t *testing.T) {
	c := NewCache(10)
	c.Put("potat", "tomat")
	x, ok := c.Get("potat")
	if !ok {
		t.Error("fail the first potat tomat test")
	}
	if x != "tomat" {
		t.Errorf("failed the potat tomat test")
	}
}

func TestCacheRace(t *testing.T) {
	var wait sync.WaitGroup
	wait.Add(10)
	cache := NewCache(100)
	cache.Put("foo", "init")
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				if j%2 == 0 {
					cache.Get("foo")
				}
				cache.Put("foo", strconv.Itoa(j))
			}
			wait.Done()
		}()
	}
	t.Log("waiting")
	wait.Wait()
	t.Log("done")

}
