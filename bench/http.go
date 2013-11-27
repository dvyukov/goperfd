// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"time"
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

func benchmarkHttp(N uint64) {
	PerfParallel(N, 4, func() {
		t0 := time.Now()
		res, err := http.Get(ts.URL)
		if err != nil {
			log.Printf("Get: %v", err)
			return
		}
		all, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("ReadAll: %v", err)
			return
		}
		body := string(all)
		if body != "Hello world.\n" {
			log.Fatalf("Got body: " + body)
		}
		PerfLatencyNote(t0)
	})
}
