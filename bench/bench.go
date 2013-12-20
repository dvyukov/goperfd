// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"code.google.com/p/goperfd/bench/driver"

	_ "code.google.com/p/goperfd/bench/build"
	_ "code.google.com/p/goperfd/bench/json"
	_ "code.google.com/p/goperfd/bench/http"
)

func main() {
	driver.Main()
}
