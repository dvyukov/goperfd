// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build linux

package driver

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
)

const rssMultiplier = 1 << 10

// Runs the cmd under perf. Returns filename of the profile. Any errors are ignored.
func RunUnderProfiler(args ...string) (string, string) {
	cmd := exec.Command("perf", append([]string{"record", "-o", "perf.data"}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("Failed to execute 'perf record %v': %v\n%v", args, err, string(out))
		return "", ""
	}

	perf1 := perfReport("--sort", "comm")
	perf2 := perfReport()
	return perf1, perf2
}

func perfReport(args ...string) string {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("perf", append([]string{"report", "--stdio"}, args...)...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to execute 'perf report': %v\n%v", err, stderr.String())
		return ""
	}

	f, err := os.Create(tempFilename("perf.txt"))
	if err != nil {
		log.Printf("Failed to create profile file: %v", err)
		return ""
	}
	defer f.Close()

	ff := bufio.NewWriter(f)
	defer ff.Flush()

	// Strip lines starting with #, and limit output to 100 lines.
	s := bufio.NewScanner(&stdout)
	for n := 0; s.Scan() && n < 100; {
		ln := s.Bytes()
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		ff.Write(ln)
		ff.Write([]byte{'\n'})
		n++
	}
	if s.Err() != nil {
		log.Printf("Failed to scan profile: %v", s.Err())
		return ""
	}

	return f.Name()
}

// Runs size on the file. Returns filename with output. Any errors are ignored.
func RunSize(file string) string {
	resf, err := os.Create(tempFilename("size.txt"))
	if err != nil {
		log.Printf("Failed to create output file: %v", err)
		return ""
	}
	defer resf.Close()

	var stderr bytes.Buffer
	cmd := exec.Command("size", "-A", file)
	cmd.Stdout = resf
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to execute 'size -m %v': %v\n%v", file, err, stderr.String())
		return ""
	}

	return resf.Name()
}

func getVMPeak() uint64 {
	data, err := ioutil.ReadFile("/proc/self/status")
	if err != nil {
		log.Printf("Failed to read /proc/self/status: %v", err)
		return 0
	}

	re := regexp.MustCompile("VmPeak:[ \t]*([0-9]+) kB")
	match := re.FindSubmatch(data)
	if match == nil {
		log.Printf("No VmPeak in /proc/self/status")
		return 0
	}
	v, err := strconv.ParseUint(string(match[1]), 10, 64)
	if err != nil {
		log.Printf("Failed to parse VmPeak in /proc/self/status: %v", string(match[1]))
		return 0
	}
	return v * 1024
}

func setProcessAffinity(v int) {
	nr := uintptr(0)
	switch runtime.GOARCH {
	case "amd64":
		nr = 203
	case "386":
		nr = 241
	default:
		log.Printf("setProcessAffinity: unsupported arch")
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	_, _, errno := syscall.Syscall(nr, uintptr(syscall.Getpid()), uintptr(unsafe.Sizeof(v)), uintptr(unsafe.Pointer(&v)))
	if errno != 0 {
		log.Printf("failed to set affinity to %v: %v", v, errno.Error())
		return
	}
	var args []string
	for i := 0; i < len(os.Args); i++ {
		a := os.Args[i]
		if strings.HasPrefix(a, "-affinity") {
			if a == "-affinity" {
				i++ // also skip the value
			}
			continue
		}
		args = append(args, a)
	}
	if err := syscall.Exec(os.Args[0], args, os.Environ()); err != nil {
		log.Printf("failed to exec: %v", err)
	}
}
