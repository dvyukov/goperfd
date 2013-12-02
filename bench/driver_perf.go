// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
)

// Runs the cmd under perf. Returns filename of the profile. Any errors are ignored.
func runUnderProfiler(args ...string) string {
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
