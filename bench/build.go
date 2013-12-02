// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"time"
)

func init() {
	RegisterBenchmark("build", BenchmarkBuild)
}

func BenchmarkBuild() PerfResult {
	if os.Getenv("GOMAXPROCS") == "" {
		os.Setenv("GOMAXPROCS", "1")
	}
	res := MakePerfResult()
	for i := 0; i < *benchNum; i++ {
		res1 := BenchmarkOnce()
		if res.RunTime == 0 || res.RunTime > res1.RunTime {
			res = res1
		}
		log.Printf("Run %v: %+v\n", i, res)
	}
	return res
}

func BenchmarkOnce() PerfResult {
	// run 'go build -a'
	t0 := time.Now()
	cmd := exec.Command("go", "install", "-a", "-p", os.Getenv("GOMAXPROCS"), "cmd/go")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start 'go install -a cmd/go': %v", err)
	}
	ss := PerfInitSysStats(1, cmd)
	if err := cmd.Wait(); err != nil {
		log.Fatalf("Failed to run 'go install -a cmd/go': %v\n%v", err, stderr.String())
	}
	res := MakePerfResult()
	res.RunTime = uint64(time.Since(t0))
	res.Metrics["runtime"] = res.RunTime
	ss.Collect(&res)

	// go command binary size
	gof, err := os.Open(os.Getenv("GOROOT") + "/bin/go")
	if err != nil {
		log.Fatalf("Failed to open $GOROOT/bin/go: %v\n", err)
	}
	st, err := gof.Stat()
	if err != nil {
		log.Fatalf("Failed to stat $GOROOT/bin/go: %v\n", err)
	}
	res.Metrics["binary-size"] = uint64(st.Size())
	return res
}
