package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func init() {
	//RegisterBenchmark("build", BenchmarkBuild)
}

type Result struct {
	runTime    uint64
	cpuTime    uint64
	binarySize uint64
	RSS        uint64
}

func BenchmarkBuild() {
	if os.Getenv("GOMAXPROCS") == "" {
		os.Setenv("GOMAXPROCS", "1")
	}
	var res Result
	for i := 0; i < *benchNum; i++ {
		res1 := BenchmarkOnce()
		if res.runTime == 0 || res.runTime > res1.runTime {
			res = res1
		}
		log.Printf("Run %v: %+v\n", i, res)
	}
	PrintMetric("runtime", res.runTime)
	PrintMetric("cputime", res.cpuTime)
	PrintMetric("binary-size", res.binarySize)
	PrintMetric("rss", res.RSS)
}

func BenchmarkOnce() (res Result) {
	// run 'go build -a'
	t0 := time.Now()
	cmd := exec.Command("go", "build", "-a", "-v", "-p", os.Getenv("GOMAXPROCS"), "std")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("Failed to run 'go build -a -v std': %v\n%v", err, string(out))
	}
	res.runTime = uint64(time.Since(t0))

	// RSS of 'go build -a'
	usage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	res.RSS = MaxRss(usage)
	res.cpuTime = CpuTime(usage)

	// go command binary size
	gof, err := os.Open(os.Getenv("GOROOT") + "/bin/go")
	if err != nil {
		log.Fatalf("Failed to open $GOROOT/bin/go: %v\n", err)
	}
	st, err := gof.Stat()
	if err != nil {
		log.Fatalf("Failed to stat $GOROOT/bin/go: %v\n", err)
	}
	res.binarySize = uint64(st.Size())
	return
}
