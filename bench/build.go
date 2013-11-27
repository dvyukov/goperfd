// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
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
	cmd := exec.Command("go", "build", "-a", "-v", "-p", os.Getenv("GOMAXPROCS"), "std")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run 'go build -a -v std': %v\n%v", err, string(out))
	}
	res := MakePerfResult()
	res.RunTime = uint64(time.Since(t0))
	res.Metrics["runtime"] = res.RunTime

	// RSS of 'go build -a'
	usage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	res.Metrics["rss"] = MaxRss(usage)
	res.Metrics["cputime"] = CpuTime(usage)

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
