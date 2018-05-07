package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var wg sync.WaitGroup
var fs int32

type result struct {
	val       string
	expiresAt time.Time
	c         *sync.Cond
	isRunning bool
}

type dedup struct {
	resultsMu *sync.Mutex
	limit     time.Duration
	f         func(string) string
	results   map[string]*result
}

func newDedup(limit time.Duration, f func(string) string) *dedup {
	return &dedup{
		resultsMu: new(sync.Mutex),
		limit:     limit,
		f:         f,
		results:   make(map[string]*result),
	}
}

func (d *dedup) run(x string) string {
	d.resultsMu.Lock()
	r, ok := d.results[x]
	if !ok {
		mu := new(sync.Mutex)
		c := sync.NewCond(mu)
		r = &result{c: c, isRunning: false}
		d.results[x] = r
	}
	d.resultsMu.Unlock()

	if !time.Now().After(r.expiresAt) {
		return r.val
	}

	r.c.L.Lock()

	for r.isRunning {
		r.c.Wait()
	}

	if !time.Now().After(r.expiresAt) {
		r.c.L.Unlock()
		r.c.Broadcast()
		return r.val
	}

	r.isRunning = true
	r.val = d.f(x)
	r.expiresAt = time.Now().Add(d.limit)
	r.isRunning = false
	r.c.L.Unlock()
	r.c.Broadcast()

	return r.val
}

func main() {
	// logger := log.New(os.Stdout, "func-dedup", log.Ldate|log.Ltime|log.Lmicroseconds|log.Llongfile)

	d := newDedup(time.Millisecond*100, func(s string) string {
		atomic.AddInt32(&fs, 1)

		// r := rand.New(rand.NewSource(time.Now().UnixNano()))
		// n := r.Int31n(10)

		// logger.Printf("Executing F, calling d.run; first sleeping for %d seconds\n", n)
		// time.Sleep(time.Duration(n) * time.Second)

		return s
	})

	// Multiple competing goroutines so that there is lock contention
	for i := 0; i < 1000; i = i + 1 {
		if i%100 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
		input := "G"
		wg.Add(1)
		// logger.Println("generating goroutine", i)
		go func(s string, id int) {
			d.run(s)
			// logger.Println(id, "done executing dedup")
			defer wg.Done()
		}(input, i)
	}

	wg.Wait()

	// logger.Println("A	ll done")

	ffs := atomic.LoadInt32(&fs)

	fmt.Println("Calls to f", ffs)
}
