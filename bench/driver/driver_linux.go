// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package driver

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

const rssMultiplier = 1 << 10

// Runs the cmd under perf. Returns filename of the profile. Any errors are ignored.
func RunUnderProfiler(args ...string) string {
	perf, err := os.Create(tempFilename("perf.txt"))
	if err != nil {
		log.Printf("Failed to create profile file: %v", err)
		return ""
	}
	defer perf.Close()

	cmd := exec.Command("perf", append([]string{"record"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to execute 'perf record %v': %v\n%v", cmd, err, string(out))
		return ""
	}

	var stderr bytes.Buffer
	cmd = exec.Command("perf", "report", "--stdio", "--sort", "comm")
	cmd.Stdout = perf
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		log.Printf("Failed to execute 'perf report': %v\n%v", err, stderr.String())
		return ""
	}

	stderr.Reset()
	cmd = exec.Command("perf", "report", "--stdio")
	cmd.Stdout = perf
	cmd.Stderr = &stderr
	if err = cmd.Run(); err != nil {
		log.Printf("Failed to execute 'perf report': %v\n%v", err, stderr.String())
		return ""
	}

	return perf.Name()
}

func getVMPeak(pid int) uint64 {
	pids := "self"
	if pid != 0 {
		pids = strconv.Itoa(pid)
	}
	data, err := ioutil.ReadFile(fmt.Sprintf("/proc/%v/status", pids))
	if err != nil {
		log.Printf("Failed to read /proc/pid/status: %v", err)
		return 0
	}

	re := regexp.MustCompile("VmPeak:[ \t]*([0-9]+) kB")
	match := re.FindSubmatch(data)
	if match == nil {
		log.Printf("No VmPeak in /proc/pid/status")
		return 0
	}
	v, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		log.Printf("Failed to parse VmPeak in /proc/pid/status: %v", string(match[1]))
		return 0
	}
	return v * 1024
}
