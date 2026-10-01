package ttlcache

import (
	"sync"
	"testing"
	"time"
)

func TestCacheExpires(t *testing.T) {
	c := New[int](20 * time.Millisecond)
	c.Set("k", 1)
	if v, ok := c.Get("k"); !ok || v != 1 {
		t.Fatalf("Get = %d,%v; want 1,true", v, ok)
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("entry should have expired")
	}
}

func TestCacheConcurrentGetPut(_ *testing.T) {
	c := New[int](10 * time.Minute)
	const (
		writers = 4
		reads   = 100
	)
	var wg sync.WaitGroup
	for i := range writers {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := range reads {
				key := "k" + string(rune(j%10))
				val := writerID*1000 + j
				c.Set(key, val)
			}
		}(i)
	}
	for range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range reads {
				key := "k" + string(rune(j%10))
				_, _ = c.Get(key)
			}
		}()
	}
	wg.Wait()
}

func TestCacheConcurrentPutExpiry(_ *testing.T) {
	c := New[string](50 * time.Millisecond)
	const goroutines = 8
	var wg sync.WaitGroup
	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 10 {
				c.Set("key", "value")
				_, _ = c.Get("key")
				if j == 5 {
					time.Sleep(60 * time.Millisecond)
				}
			}
		}()
	}
	wg.Wait()
}
