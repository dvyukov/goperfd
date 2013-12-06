// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/goperfd/bench/driver"

	_ "code.google.com/p/goperfd/bench/json"
	//_ "dvyukov-go-perf-dashboard/bench/rpc"
	_ "code.google.com/p/goperfd/bench/build"
	//_ "dvyukov-go-perf-dashboard/bench/http"
	//_ "dvyukov-go-perf-dashboard/bench/widefinder"
)

func main() {
	driver.Main()
}
