package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

var wg sync.WaitGroup
var fs int32

type result struct {
	val       string
	expiresAt time.Time
}

type dedup struct {
	mu      sync.Mutex
	limit   time.Duration
	f       func(string) string
	results map[string]*result
}

func newDedup(limit time.Duration, f func(string) string) *dedup {
	return &dedup{
		limit:   limit,
		f:       f,
		results: make(map[string]*result),
	}
}

func (d *dedup) run(x string) string {
	d.mu.Lock()
	// Playing around with waiting on go routines
	defer wg.Done()
	defer d.mu.Unlock()

	r, ok := d.results[x]
	if !ok {
		r = &result{}
		d.results[x] = r
	}
	if time.Now().After(r.expiresAt) {
		r.val = d.f(x)
		r.expiresAt = time.Now().Add(d.limit)
	}
	return r.val
}

func main() {
	logger := log.New(os.Stdout, "func-dedup", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)

	d := newDedup(time.Millisecond*100, func(s string) string {
		atomic.AddInt32(&fs, 1)

		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		n := r.Int31n(10)

		logger.Printf("Executing F, calling d.run; first sleeping for %d seconds\n", n)
		time.Sleep(time.Duration(n) * time.Second)

		return s
	})

	// Multiple competing goroutines so that there is lock contention
	for i := 0; i < 100; i = i + 1 {
		time.Sleep(100 * time.Millisecond)
		input := "G"
		wg.Add(1)
		logger.Println("generating goroutine", i)
		go func(s string, id int) {
			d.run(s)
			logger.Println(id, "done executing dedup")
		}(input, i)
	}

	wg.Wait()

	logger.Println("All done")

	ffs := atomic.LoadInt32(&fs)

	fmt.Println("Calls to f", ffs)
}
