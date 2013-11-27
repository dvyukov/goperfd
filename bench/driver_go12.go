// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.2

package main

import (
	"runtime"
)

// New mem stats added in Go1.2
func PerfCollectGo12MemStats(res *PerfResult, mstats0, mstats1 *runtime.MemStats) {
	res.Metrics["sys-gc"] = mstats1.GCSys
	res.Metrics["sys-other"] = mstats1.OtherSys + mstats1.MSpanSys + mstats1.MCacheSys + mstats1.BuckHashSys
}
