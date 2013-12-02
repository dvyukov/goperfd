// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"syscall"
)

func runUnderProfiler(args ...string) string {
	return ""
}

type sysStats struct {
	N      uint64
	Cmd    *exec.Cmd
	Rusage syscall.Rusage
}

func PerfInitSysStats(N uint64, cmd *exec.Cmd) sysStats {
	ss := sysStats{N: N, Cmd: cmd}
	if cmd == nil {
		// TODO:
		// 1. call syscall.GetProcessMemoryInfo(syscall.GetCurrentProcess()), save info
		// 2. call syscall.GetSystemTimes(syscall.GetCurrentProcess()), save info
	} else {
		// TODO:
		// get cmd.Process.Pid, convert it to handle with syscall.OpenProcess, save the handle
	}
	return ss
}

func (ss sysStats) Collect(res *PerfResult) {
	if ss.Cmd == nil {
		// TODO:
		// 1. call syscall.GetProcessMemoryInfo(syscall.GetCurrentProcess())
		// 2. call syscall.GetSystemTimes(syscall.GetCurrentProcess())
		// 3. calculate diffs with saved info
	} else {
		// 1. call syscall.GetProcessMemoryInfo(saved handle)
		// 2. call syscall.GetSystemTimes(saved handle)
		// 3. calculate stats
	}
	// res.Metrics["rss"] = ...
	// res.Metrics["cputime"] = ...
}

/* FTR

type syscall.Rusage struct {
        CreationTime Filetime
        ExitTime     Filetime
        KernelTime   Filetime
        UserTime     Filetime
}

func ftToDuration(ft *syscall.Filetime) time.Duration {
        n := int64(ft.HighDateTime)<<32 + int64(ft.LowDateTime) // in 100-nanosecond intervals
        return time.Duration(n*100) * time.Nanosecond
}
*/
