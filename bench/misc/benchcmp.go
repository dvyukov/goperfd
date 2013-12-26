// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	benchList = flag.String("bench", "", "comma-separated list of benchmarks to run")
	benchNum  = flag.Int("benchnum", 5, "number of benchmark runs")
	benchMem  = flag.Int("benchmem", 64, "approx RSS value to aim at in benchmarks, in MB")
	benchTime = flag.Duration("benchtime", 5*time.Second, "run enough iterations of each benchmark to take the specified time")
	benchCPU  = flag.String("benchcpu", "", "comma-separated list of GOMAXPROCS values")
	affinity  = flag.String("affinity", "", "comma-delimited list of process affinities")
	oldBin    = flag.String("old", "", "old bench binary")
	newBin    = flag.String("new", "", "new bench binary")
)

func main() {
	flag.Parse()
	if *oldBin == "" || *newBin == "" {
		fmt.Fprintf(os.Stderr, "specify old and new binary\n")
		flag.PrintDefaults()
		os.Exit(1)
	}
	if *benchList == "" {
		*benchList = "build,garbage,http,json,rpc,widefinder"
	}
	if *benchCPU == "" {
		*benchCPU = "1"
	}
	affinityList := strings.Split(*affinity, ",")
	for _, bench := range strings.Split(*benchList, ",") {
		for pi, procs := range strings.Split(*benchCPU, ",") {
			aff := ""
			if len(affinityList) > pi {
				aff = affinityList[pi]
			}
			if aff == "" {
				aff = "0"
			}
			benchCmp(bench, procs, aff)
		}
	}
}

type Metrics map[string]uint64

func benchCmp(bench, procs, aff string) {
	fmt.Printf("%v-%v\n", bench, procs)
	m0 := benchOne(*oldBin, bench, procs, aff)
	m1 := benchOne(*newBin, bench, procs, aff)

	var metrics []string
	for metric := range m0 {
		metrics = append(metrics, metric)
	}
	sort.Strings(metrics)
	for _, metric := range metrics {
		v0 := m0[metric]
		v1 := m1[metric]
		d := float64(v1)/float64(v0)*100 - 100
		fmt.Printf("%-20s %12v %12v %10v%%\n", metric, v0, v1, fmt.Sprintf("%+.2f", d))
	}
	fmt.Printf("\n")
}

func benchOne(bin, bench, procs, aff string) Metrics {
	os.Setenv("GOMAXPROCS", procs)
	cmd := exec.Command(bin,
		"-bench", bench,
		"-benchnum", strconv.Itoa(*benchNum),
		"-benchmem", strconv.Itoa(*benchMem),
		"-benchtime", benchTime.String(),
		"-affinity", aff)
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v failed: %v\n%s\n", cmd.Args, err, out)
		os.Exit(1)
	}
	metrics := make(Metrics)
	s := bufio.NewScanner(bytes.NewReader(out))
	metricRe := regexp.MustCompile("^GOPERF-METRIC:([a-z,0-9,-]+)=([0-9]+)$")
	for s.Scan() {
		ss := metricRe.FindStringSubmatch(s.Text())
		if ss == nil {
			continue
		}
		var v uint64
		v, err := strconv.ParseUint(ss[2], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse metric '%v=%v': %v", ss[1], ss[2], err)
			continue
		}
		metrics[ss[1]] = v
	}
	return metrics
}
