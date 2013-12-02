// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux

package main

import (
	"log"
	"os/exec"
	"syscall"
)

var sysStats struct {
	N      uint64
	Rusage syscall.Rusage
}

func PerfInitSysStats(N uint64) {
	sysStats.N = N
	err := syscall.Getrusage(0, &sysStats.Rusage)
	if err != nil {
		log.Printf("Getrusage failed: %v", err)
		sysStats.N = 0
		// Deliberately ignore the error.
		return
	}
}

func PerfCollectSysStats(res *PerfResult) {
	if sysStats.N == 0 {
		return
	}
	Rusage := new(syscall.Rusage)
	err := syscall.Getrusage(0, Rusage)
	if err != nil {
		log.Printf("Getrusage failed: %v", err)
		// Deliberately ignore the error.
		return
	}
	res.Metrics["rss"] = maxRss(Rusage)
	res.Metrics["cputime"] = (cpuTime(Rusage) - cpuTime(&sysStats.Rusage)) / sysStats.N
}

func PerfCollectProcessStats(res *PerfResult, cmd *exec.Cmd) {
	usage := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	res.Metrics["rss"] = maxRss(usage)
	res.Metrics["cputime"] = cpuTime(usage)
}

func cpuTime(usage *syscall.Rusage) uint64 {
	return uint64(usage.Utime.Sec)*1e9 + uint64(usage.Utime.Usec*1e3) +
		uint64(usage.Stime.Sec)*1e9 + uint64(usage.Stime.Usec)*1e3
}

func maxRss(usage *syscall.Rusage) uint64 {
	return uint64(usage.Maxrss) * (1 << 10)
}
