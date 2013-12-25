// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package driver

import (
	"log"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

// access to Windows APIs
var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	modpsapi                     = syscall.NewLazyDLL("psapi.dll")
	procGetProcessMemoryInfo     = modpsapi.NewProc("GetProcessMemoryInfo")
	procSetProcessAffinityMask   = modkernel32.NewProc("SetProcessAffinityMask")
	procCreateJobObjectW         = modkernel32.NewProc("CreateJobObjectW")
	procOpenJobObjectW           = modkernel32.NewProc("OpenJobObjectW")
	procAssignProcessToJobObject = modkernel32.NewProc("AssignProcessToJobObject")
	procSetInformationJobObject  = modkernel32.NewProc("SetInformationJobObject")

	currentProcess syscall.Handle
	childMu        sync.Mutex
	childProcesses []syscall.Handle
)

const (
	JOB_OBJECT_MSG_NEW_PROCESS                  = 6
	JobObjectAssociateCompletionPortInformation = 7
)

type PROCESS_MEMORY_COUNTERS struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

type JOBOBJECT_ASSOCIATE_COMPLETION_PORT struct {
	CompletionKey  uintptr
	CompletionPort syscall.Handle
}

func init() {
	// Create Job object and assign current process to it.
	var err error
	currentProcess, err = syscall.GetCurrentProcess()
	if err != nil {
		log.Fatalf("GetCurrentProcess failed: %v", err)
	}
	jobObject, err := createJobObject(nil, nil)
	if err != nil {
		log.Printf("CreateJobObject failed: %v", err)
		return
	}
	if err = assignProcessToJobObject(jobObject, currentProcess); err != nil {
		log.Printf("AssignProcessToJobObject failed: %v", err)
		syscall.Close(jobObject)
		return
	}
	iocp, err := syscall.CreateIoCompletionPort(syscall.InvalidHandle, 0, 0, 1)
	if err != nil {
		log.Printf("CreateIoCompletionPort failed: %v", err)
		syscall.Close(jobObject)
		return
	}
	port := JOBOBJECT_ASSOCIATE_COMPLETION_PORT{
		CompletionKey:  uintptr(jobObject),
		CompletionPort: iocp,
	}
	err = setInformationJobObject(jobObject, JobObjectAssociateCompletionPortInformation, uintptr(unsafe.Pointer(&port)), uint32(unsafe.Sizeof(port)))
	if err != nil {
		log.Printf("SetInformationJobObject failed: %v", err)
		syscall.Close(jobObject)
		syscall.Close(iocp)
		return
	}
	// Read Job notifications about new "child" processes and collect them in childProcesses.
	go func() {
		var code, key uint32
		var o *syscall.Overlapped
		for {
			err := syscall.GetQueuedCompletionStatus(iocp, &code, &key, &o, syscall.INFINITE)
			if err != nil {
				log.Printf("GetQueuedCompletionStatus failed: %v", err)
				return
			}
			if key != uint32(jobObject) {
				panic("Invalid GetQueuedCompletionStatus key parameter")
			}
			if code == JOB_OBJECT_MSG_NEW_PROCESS {
				pid := int(uintptr(unsafe.Pointer(o)))
				if pid == syscall.Getpid() {
					continue
				}
				c, err := syscall.OpenProcess(syscall.PROCESS_QUERY_INFORMATION, false, uint32(pid))
				if err != nil {
					log.Printf("OpenProcess failed: %v", err)
					continue
				}
				childMu.Lock()
				childProcesses = append(childProcesses, c)
				childMu.Unlock()
			}
		}
	}()
}

func getProcessMemoryInfo(h syscall.Handle, mem *PROCESS_MEMORY_COUNTERS) (err error) {
	r1, _, e1 := syscall.Syscall(procGetProcessMemoryInfo.Addr(), 3, uintptr(h), uintptr(unsafe.Pointer(mem)), uintptr(unsafe.Sizeof(*mem)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func setProcessAffinityMask(h syscall.Handle, mask uintptr) (err error) {
	r1, _, e1 := syscall.Syscall(procSetProcessAffinityMask.Addr(), 2, uintptr(h), mask, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func createJobObject(jobAttrs *syscall.SecurityAttributes, name *uint16) (handle syscall.Handle, err error) {
	r0, _, e1 := syscall.Syscall(procCreateJobObjectW.Addr(), 2, uintptr(unsafe.Pointer(jobAttrs)), uintptr(unsafe.Pointer(name)), 0)
	handle = syscall.Handle(r0)
	if handle == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func assignProcessToJobObject(job syscall.Handle, process syscall.Handle) (err error) {
	r1, _, e1 := syscall.Syscall(procAssignProcessToJobObject.Addr(), 2, uintptr(job), uintptr(process), 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func setInformationJobObject(job syscall.Handle, infoclass uint32, info uintptr, infolien uint32) (err error) {
	r1, _, e1 := syscall.Syscall6(procSetInformationJobObject.Addr(), 4, uintptr(job), uintptr(infoclass), uintptr(info), uintptr(infolien), 0, 0)
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func RunUnderProfiler(args ...string) (string, string) {
	return "", ""
}

// Size runs size command on the file. Returns filename with output. Any errors are ignored.
func Size(file string) string {
	return ""
}

type sysStats struct {
	N       uint64
	Current bool
	CPU     syscall.Rusage
}

func InitSysStats(N uint64, cmd *exec.Cmd) (ss sysStats) {
	if cmd == nil {
		ss.Current = true
		if err := syscall.GetProcessTimes(currentProcess, &ss.CPU.CreationTime, &ss.CPU.ExitTime, &ss.CPU.KernelTime, &ss.CPU.UserTime); err != nil {
			log.Printf("GetProcessTimes failed: %v", err)
			return
		}
	} else {
		childMu.Lock()
		children := childProcesses
		childProcesses = []syscall.Handle{}
		childMu.Unlock()
		for _, proc := range children {
			syscall.CloseHandle(proc)
		}
	}
	ss.N = N
	return ss
}

func (ss sysStats) Collect(res *Result, prefix string) {
	if ss.N == 0 {
		return
	}
	if ss.Current {
		var Mem PROCESS_MEMORY_COUNTERS
		if err := getProcessMemoryInfo(currentProcess, &Mem); err != nil {
			log.Printf("GetProcessMemoryInfo failed: %v", err)
			return
		}
		var CPU syscall.Rusage
		if err := syscall.GetProcessTimes(currentProcess, &CPU.CreationTime, &CPU.ExitTime, &CPU.KernelTime, &CPU.UserTime); err != nil {
			log.Printf("GetProcessTimes failed: %v", err)
			return
		}
		res.Metrics[prefix+"cputime"] = (getCPUTime(CPU) - getCPUTime(ss.CPU)) / ss.N
		res.Metrics[prefix+"rss"] = uint64(Mem.PeakWorkingSetSize)
	} else {
		childMu.Lock()
		children := childProcesses
		childProcesses = []syscall.Handle{}
		childMu.Unlock()
		if len(children) == 0 {
			log.Printf("sysStats.Collect: no child processes?")
			return
		}
		defer func() {
			for _, proc := range children {
				syscall.CloseHandle(proc)
			}
		}()
		cputime := uint64(0)
		rss := uint64(0)
		for _, proc := range children {
			var Mem PROCESS_MEMORY_COUNTERS
			if err := getProcessMemoryInfo(proc, &Mem); err != nil {
				log.Printf("GetProcessMemoryInfo failed: %v", err)
				return
			}
			var CPU syscall.Rusage
			if err := syscall.GetProcessTimes(proc, &CPU.CreationTime, &CPU.ExitTime, &CPU.KernelTime, &CPU.UserTime); err != nil {
				log.Printf("GetProcessTimes failed: %v", err)
				return
			}
			cputime += getCPUTime(CPU) / ss.N
			rss += uint64(Mem.PeakWorkingSetSize)
		}
		res.Metrics[prefix+"cputime"] = cputime
		res.Metrics[prefix+"rss"] = rss
	}
}

func getCPUTime(CPU syscall.Rusage) uint64 {
	var CPU0 syscall.Rusage // time is offsetted, so we need to subtract "zero"
	return uint64(CPU.KernelTime.Nanoseconds() + CPU.UserTime.Nanoseconds() -
		CPU0.KernelTime.Nanoseconds() - CPU0.UserTime.Nanoseconds())
}

func setProcessAffinity(v int) {
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		log.Printf("GetCurrentProcess failed: %v", err)
		return
	}
	if err := setProcessAffinityMask(h, uintptr(v)); err != nil {
		log.Printf("SetProcessAffinityMask failed: %v", err)
		return
	}
}
