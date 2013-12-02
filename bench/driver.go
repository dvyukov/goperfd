// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	bench     = flag.String("bench", "", "benchmark to run")
	flake     = flag.Int("flake", 0, "test flakiness of a benchmark")
	benchNum  = flag.Int("benchnum", 3, "run each benchmark that many times")
	benchTime = flag.Duration("benchtime", 10*time.Second, "benchmarking time for a single run")
	benchMem  = flag.Int("benchmem", 64, "approx RSS value to aim at in benchmarks, in MB")
	tmpDir    = flag.String("tmpdir", os.TempDir(), "dir for temporary files")
)

var benchmarks = make(map[string]func() PerfResult)

func RegisterBenchmark(name string, f func() PerfResult) {
	benchmarks[name] = f
}

func main() {
	flag.Parse()
	if *bench == "" {
		printBenchmarks()
		return
	}
	f := benchmarks[*bench]
	if f == nil {
		fmt.Printf("unknown benchmark '%v'\n", *bench)
		os.Exit(1)
	}
	if *flake > 0 {
		testFlakiness(f, *flake)
		return
	}
	res := f()
	for k, v := range res.Metrics {
		fmt.Printf("GOPERF-METRIC:%v=%v\n", k, v)
	}
	for k, v := range res.Files {
		fmt.Printf("GOPERF-FILE:%v=%v\n", k, v)
	}
}

func printBenchmarks() {
	var bb []string
	for name, _ := range benchmarks {
		bb = append(bb, name)
	}
	sort.Strings(bb)
	for i, name := range bb {
		if i != 0 {
			fmt.Print(",")
		}
		fmt.Print(name)
	}
	fmt.Print("\n")
}

func testFlakiness(f func() PerfResult, N int) {
	res := make([]PerfResult, N+2)
	for i := range res {
		res[i] = f()
	}
	fmt.Printf("\n")
	for k, v := range res[0].Metrics {
		fmt.Printf("%v:\t", k)
		for i := 2; i < len(res); i++ {
			d := 100*float64(v)/float64(res[i].Metrics[k]) - 100
			fmt.Printf(" %+.2f%%", d)
		}
		fmt.Printf("\n")
	}
}

type PerfResult struct {
	N        uint64        // number of iterations
	Duration time.Duration // total run duration
	RunTime  uint64        // ns/op
	Metrics  map[string]uint64
	Files    map[string]string
}

func MakePerfResult() PerfResult {
	return PerfResult{Metrics: make(map[string]uint64), Files: make(map[string]string)}
}

// PerfParallel is a helper function that runs f
// N times in P*GOMAXPROCS goroutines.
func PerfParallel(N uint64, P int, f func()) {
	numProcs := P * runtime.GOMAXPROCS(0)
	var wg sync.WaitGroup
	wg.Add(numProcs)
	for p := 0; p < numProcs; p++ {
		go func() {
			for int64(atomic.AddUint64(&N, ^uint64(0))) >= 0 {
				f()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

type BenchFunc func(N uint64)

func PerfBenchmark(f BenchFunc) PerfResult {
	res := MakePerfResult()
	for i := 0; i < *benchNum; i++ {
		res1 := RunBenchmark(f)
		if res.N == 0 || res.RunTime > res1.RunTime {
			if res.N != 0 {
				for k, v := range res.Metrics {
					if k == "rss" || strings.HasPrefix(k, "sys-") {
						res1.Metrics[k] = v
					}
				}
			}
			res = res1
		}
	}

	cpuprof, err := os.Create(tempFilename("cpuprof.txt"))
	if err != nil {
		log.Fatalf("Failed to create profile file: %v", err)
	}
	defer cpuprof.Close()
	var cpuproflog bytes.Buffer
	cmd := exec.Command("go", "tool", "pprof", "--text", os.Args[0], res.Files["cpuprof"])
	cmd.Stdout = cpuprof
	cmd.Stderr = &cpuproflog
	err = cmd.Run()
	delete(res.Files, "cpuprof")
	if err != nil {
		log.Printf("go tool pprof cpuprof failed: %v\n%v", err, cpuproflog.String())
		// Deliberately ignore the error and continue.
	} else {
		res.Files["cpuprof"] = cpuprof.Name()
	}

	memprof, err := os.Create(tempFilename("memprof.txt"))
	if err != nil {
		log.Fatalf("Failed to create profile file: %v", err)
	}
	defer memprof.Close()
	var memproflog bytes.Buffer
	cmd = exec.Command("go", "tool", "pprof", "--text", "--lines", "--show_bytes",
		"--alloc_space", "--base", res.Files["memprof0"], os.Args[0], res.Files["memprof"])
	cmd.Stdout = memprof
	cmd.Stderr = &memproflog
	err = cmd.Run()
	delete(res.Files, "memprof")
	delete(res.Files, "memprof0")
	if err != nil {
		log.Printf("go tool pprof memprof failed: %v\n%v", err, memproflog.String())
		// Deliberately ignore the error and continue.
	} else {
		res.Files["memprof"] = memprof.Name()
	}
	return res
}

func RunBenchmark(f BenchFunc) PerfResult {
	res := MakePerfResult()
	for ChooseN(&res) {
		log.Printf("Benchmarking %v iterations\n", res.N)
		res = RunOnce(f, res.N)
		log.Printf("Done: %+v\n", res)
	}
	return res
}

func RunOnce(f BenchFunc, N uint64) PerfResult {
	PerfLatencyInit(N)
	runtime.GC()
	mstats0 := new(runtime.MemStats)
	runtime.ReadMemStats(mstats0)
	PerfInitSysStats(N)
	res := MakePerfResult()
	res.N = N
	res.Files["memprof0"] = tempFilename("memprof")
	memprof0, err := os.Create(res.Files["memprof0"])
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.Files["memprof0"], err)
	}
	pprof.WriteHeapProfile(memprof0)
	memprof0.Close()

	res.Files["cpuprof"] = tempFilename("cpuprof")
	cpuprof, err := os.Create(res.Files["cpuprof"])
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.Files["cpuprof"], err)
	}
	defer cpuprof.Close()
	pprof.StartCPUProfile(cpuprof)
	t0 := time.Now()
	f(N)
	res.Duration = time.Since(t0)
	res.RunTime = uint64(time.Since(t0)) / N
	res.Metrics["runtime"] = res.RunTime
	pprof.StopCPUProfile()

	PerfLatencyCollect(&res)

	res.Files["memprof"] = tempFilename("memprof")
	memprof, err := os.Create(res.Files["memprof"])
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.Files["memprof"], err)
	}
	pprof.WriteHeapProfile(memprof)
	memprof.Close()

	PerfCollectSysStats(&res)
	mstats1 := new(runtime.MemStats)
	runtime.ReadMemStats(mstats1)
	res.Metrics["allocated"] = (mstats1.TotalAlloc - mstats0.TotalAlloc) / N
	res.Metrics["allocs"] = (mstats1.Mallocs - mstats0.Mallocs) / N
	res.Metrics["sys-total"] = mstats1.Sys
	res.Metrics["sys-heap"] = mstats1.HeapSys
	res.Metrics["sys-stack"] = mstats1.StackSys
	res.Metrics["gc-pause-total"] = (mstats1.PauseTotalNs - mstats0.PauseTotalNs) / N
	PerfCollectGo12MemStats(&res, mstats0, mstats1)
	numGC := uint64(mstats1.NumGC - mstats0.NumGC)
	if numGC == 0 {
		res.Metrics["gc-pause-one"] = 0
	} else {
		res.Metrics["gc-pause-one"] = (mstats1.PauseTotalNs - mstats0.PauseTotalNs) / numGC
	}
	return res
}

var perfLatency struct {
	data PerfLatencyData
	idx  int32
}

type PerfLatencyData []uint64

func (p PerfLatencyData) Len() int           { return len(p) }
func (p PerfLatencyData) Less(i, j int) bool { return p[i] < p[j] }
func (p PerfLatencyData) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func PerfLatencyInit(N uint64) {
	N = min(N, 1e6) // bound the amount of memory consumed
	perfLatency.data = make(PerfLatencyData, N)
	perfLatency.idx = 0
}

func PerfLatencyNote(t time.Time) {
	d := time.Since(t)
	i := atomic.AddInt32(&perfLatency.idx, 1) - 1
	if int(i) >= len(perfLatency.data) {
		return
	}
	perfLatency.data[i] = uint64(d)
}

func PerfLatencyCollect(res *PerfResult) {
	cnt := perfLatency.idx
	if cnt == 0 {
		return
	}
	sort.Sort(perfLatency.data[:cnt])
	res.Metrics["latency-50"] = perfLatency.data[cnt*50/100]
	res.Metrics["latency-95"] = perfLatency.data[cnt*95/100]
	res.Metrics["latency-99"] = perfLatency.data[cnt*99/100]
}

func ChooseN(res *PerfResult) bool {
	const MaxN = 1e12
	last := res.N
	if last == 0 {
		res.N = 1
		return true
	} else if res.Duration >= *benchTime || last >= MaxN {
		return false
	}
	nsPerOp := max(1, res.RunTime)
	res.N = uint64(*benchTime) / nsPerOp
	res.N = max(min(res.N+res.N/2, 100*last), last+1)
	res.N = roundUp(res.N)
	return true
}

func roundUp(n uint64) uint64 {
	tmp := n
	base := uint64(1)
	for tmp >= 10 {
		tmp /= 10
		base *= 10
	}
	switch {
	case n <= base:
		return base
	case n <= (2 * base):
		return 2 * base
	case n <= (5 * base):
		return 5 * base
	default:
		return 10 * base
	}
	panic("unreachable")
	return 0
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

var tmpSeq = 0

func tempFilename(ext string) string {
	tmpSeq++
	return fmt.Sprintf("%v/%v.%v", *tmpDir, tmpSeq, ext)
}
