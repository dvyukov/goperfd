package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"runtime/pprof"
	"sort"
	"strings"
	"syscall"
	"time"
)

var (
	bench     = flag.String("bench", "", "benchmark to run")
	benchNum  = flag.Int("benchnum", 3, "run each benchmark that many times")
	benchTime = flag.Duration("benchtime", 10*time.Second, "benchmarking time for a single run")
	benchMem  = flag.Int("benchmem", 64, "approx RSS value to aim at in benchmarks, in MB")
	tmpDir    = flag.String("tmpdir", os.TempDir(), "dir for temporary files")
)

var benchmarks = make(map[string]func())

func RegisterBenchmark(name string, f func()) {
	benchmarks[name] = f
}

func main() {
	flag.Parse()
	if *bench == "" {
		PrintBenchmarks()
		return
	}
	f := benchmarks[*bench]
	if f == nil {
		fmt.Printf("unknown benchmark '%v'\n", *bench)
		os.Exit(1)
	}
	f()
}

func PrintBenchmarks() {
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

type PerfResult struct {
	N        uint64
	Duration time.Duration
	RunTime  uint64
	Metrics  map[string]uint64
	CpuProf  string
	MemProf0 string
	MemProf1 string
}

type BenchFunc func(N uint64) (map[string]uint64, error)

func PerfBenchmark(f BenchFunc) {
	res := PerfResult{Metrics: make(map[string]uint64)}
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
	for k, v := range res.Metrics {
		PrintMetric(k, v)
	}

	cpuprof, err := os.Create(tempFilename("cpuprof.txt"))
	if err != nil {
		log.Fatalf("Failed to create profile file: %v", err)
	}
	defer cpuprof.Close()
	cmd := exec.Command("go", "tool", "pprof", "--text", "--lines", os.Args[0], res.CpuProf)
	cmd.Stdout = cpuprof
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute go tool pprof: %v", err)
	}
	fmt.Printf("GOPERF-FILE:cpuprof=%v\n", cpuprof.Name())

	memprof, err := os.Create(tempFilename("memprof.txt"))
	if err != nil {
		log.Fatalf("Failed to create profile file: %v", err)
	}
	defer memprof.Close()
	cmd = exec.Command("go", "tool", "pprof", "--text", "--lines", "--show_bytes", "--alloc_space", "--base", res.MemProf0, os.Args[0], res.MemProf1)
	cmd.Stdout = memprof
	err = cmd.Run()
	if err != nil {
		log.Fatalf("Failed to execute go tool pprof: %v", err)
	}
	fmt.Printf("GOPERF-FILE:memprof=%v\n", memprof.Name())
}

func PrintMetric(name string, val uint64) {
	fmt.Printf("GOPERF-METRIC:%v=%v\n", name, val)
}

func CpuTime(usage *syscall.Rusage) uint64 {
	return uint64(usage.Utime.Sec)*1e9 + uint64(usage.Utime.Usec*1e3) +
		uint64(usage.Stime.Sec)*1e9 + uint64(usage.Stime.Usec)*1e3
}

func MaxRss(usage *syscall.Rusage) uint64 {
	return uint64(usage.Maxrss) * (1 << 10)
}

func RunBenchmark(f BenchFunc) PerfResult {
	res := PerfResult{Metrics: make(map[string]uint64)}
	for ChooseN(&res) {
		log.Printf("Benchmarking %v iterations\n", res.N)
		res = RunOnce(f, res.N)
		log.Printf("Done: %+v\n", res)
	}
	return res
}

func RunOnce(f BenchFunc, N uint64) PerfResult {
	runtime.GC()
	mstats0 := new(runtime.MemStats)
	runtime.ReadMemStats(mstats0)
	usage0 := new(syscall.Rusage)
	err := syscall.Getrusage(0, usage0)
	if err != nil {
		log.Fatalf("Getrusage failed: %v", err)
	}
	res := PerfResult{N: N, Metrics: make(map[string]uint64)}
	res.MemProf0 = tempFilename("memprof")
	memprof0, err := os.Create(res.MemProf0)
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.MemProf0, err)
	}
	pprof.WriteHeapProfile(memprof0)
	memprof0.Close()

	res.CpuProf = tempFilename("cpuprof")
	cpuprof, err := os.Create(res.CpuProf)
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.CpuProf, err)
	}
	defer cpuprof.Close()
	pprof.StartCPUProfile(cpuprof)
	t0 := time.Now()
	metrics, err := f(N)
	if err != nil {
		log.Fatalf("Benchmark function failed: %v", err)
	}
	res.Duration = time.Since(t0)
	res.RunTime = uint64(time.Since(t0)) / N
	res.Metrics["runtime"] = res.RunTime
	pprof.StopCPUProfile()

	res.MemProf1 = tempFilename("memprof")
	memprof1, err := os.Create(res.MemProf1)
	if err != nil {
		log.Fatalf("Failed to create profile file '%v': %v", res.MemProf1, err)
	}
	pprof.WriteHeapProfile(memprof1)
	memprof1.Close()

	// RSS
	usage1 := new(syscall.Rusage)
	err = syscall.Getrusage(0, usage1)
	if err != nil {
		log.Fatalf("Getrusage failed: %v", err)
	}
	res.Metrics["rss"] = MaxRss(usage1)
	res.Metrics["cputime"] = (CpuTime(usage1) - CpuTime(usage0)) / N

	mstats1 := new(runtime.MemStats)
	runtime.ReadMemStats(mstats1)
	res.Metrics["allocated"] = (mstats1.TotalAlloc - mstats0.TotalAlloc) / N
	res.Metrics["allocs"] = (mstats1.Mallocs - mstats0.Mallocs) / N
	res.Metrics["sys-total"] = mstats1.Sys
	res.Metrics["sys-heap"] = mstats1.HeapSys
	res.Metrics["sys-stack"] = mstats1.StackSys
	res.Metrics["sys-gc"] = mstats1.GCSys
	res.Metrics["sys-other"] = mstats1.OtherSys + mstats1.MSpanSys + mstats1.MCacheSys + mstats1.BuckHashSys
	res.Metrics["gc-pause-total"] = (mstats1.PauseTotalNs - mstats0.PauseTotalNs) / N
	numGC := uint64(mstats1.NumGC - mstats0.NumGC)
	if numGC == 0 {
		res.Metrics["gc-pause-one"] = 0
	} else {
		res.Metrics["gc-pause-one"] = (mstats1.PauseTotalNs - mstats0.PauseTotalNs) / numGC
	}

	for k, v := range metrics {
		res.Metrics[k] = v
	}
	return res
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
