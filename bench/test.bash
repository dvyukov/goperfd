#!/usr/bin/env bash
# Copyright 20013 The Go Authors. All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.

set -eu

if [ -z "$GOROOT" ]; then
	echo GOROOT is not defined
	exit 1
fi

if [ ! -e "./bench.go" ]; then
	echo run from benchmark dir
	exit 1
fi

export PATH=$GOROOT/bin:$PATH
export CGO_ENABLED=0

checkrev() {
	echo checking $1_$2 @$3...
	export GOOS=$1
	export GOARCH=$2
	rm -f $GOROOT/src/pkg/runtime/z*.{c,s}
	(cd $GOROOT && hg update -r $3)
	(cd $GOROOT/src && ./make.bash)
	go build
	if [ "$GOOS" == "`go env GOHOSTOS`" -a "$GOARCH" == "`go env GOHOSTARCH`" ]; then
		./bench
		for BENCH in json; do
			./bench -bench=$BENCH -benchtime=1ms -benchnum=1
		done
	fi
}

check() {
	checkrev $1 $2 go1
	checkrev $1 $2 1c941164715000e7a24eab267b74a40402d49507  # Nov 1, 2012
	checkrev $1 $2 go1.1
	checkrev $1 $2 fc3ae28f77f950db24b08c783243df5a10fd8930  # Jun 1, 2013
	checkrev $1 $2 tip
}

check windows amd64
check linux amd64

(cd $GOROOT && hg update)
(cd $GOROOT/src && ./make.bash)
echo OK
