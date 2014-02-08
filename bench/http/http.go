// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package http

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"code.google.com/p/goperfd/bench/driver"
)

func init() {
	driver.Register("http", benchmark)
}

func benchmark() driver.Result {
	return driver.Benchmark(benchmarkHttpImpl)
}

func benchmarkHttpImpl(N uint64) {
	driver.Parallel(N, 4, func() {
		t0 := time.Now()
		res, err := http.Get(server.Addr)
		if err != nil {
			log.Printf("Get: %v", err)
			return
		}
		defer res.Body.Close()
		all, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Printf("ReadAll: %v", err)
			return
		}
		body := string(all)
		if body != "Hello world.\n" {
			log.Fatalf("Got body: " + body)
		}
		driver.LatencyNote(t0)
	})
}

var server = func() *http.Server {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if l, err = net.Listen("tcp6", "[::1]:0"); err != nil {
			log.Fatalf("failed to listen: %v", err)
		}
	}
	s := &http.Server{
		Addr:           "http://" + l.Addr().String(),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "Hello world.\n")
		}),
	}
	go s.Serve(l)
	time.Sleep(100 * time.Millisecond)
	return s
}()
