package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"sync/atomic"
)

func init() {
	RegisterBenchmark("http", BenchmarkHttp)
}

func BenchmarkHttp() PerfResult {
	return PerfBenchmark(benchmarkHttp)
}

var ts = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(rw, "Hello world.\n")
}))

func benchmarkHttp(N uint64) (metrics map[string]uint64, err error) {
	numProcs := 4 * runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(numProcs)
	for p := 0; p < numProcs; p++ {
		go func() {
			for int64(atomic.AddUint64(&N, ^uint64(0))) >= 0 {
				res, err := http.Get(ts.URL)
				if err != nil {
					log.Printf("Get: %v", err)
					continue
				}
				all, err := ioutil.ReadAll(res.Body)
				if err != nil {
					log.Printf("ReadAll: %v", err)
					continue
				}
				body := string(all)
				if body != "Hello world.\n" {
					log.Fatalf("Got body: " + body)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
	return
}
