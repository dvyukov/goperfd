// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"log"
	"net"
	"net/rpc"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"code.google.com/p/goperfd/bench/driver"
)

func init() {
	driver.Register("rpc", benchmark)
}

var rpcServerAddr string

func benchmark() driver.Result {
	if rpcServerAddr == "" {
		rpc.Register(new(Server))
		rpc.RegisterName("Server", new(Server))
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatalf("net.Listen tcp :0: %v", err)
		}
		rpcServerAddr = l.Addr().String()
		go rpc.Accept(l)
	}
	return driver.Benchmark(benchmarkN)
}

func benchmarkN(N uint64) {
	const (
		clientsPerConn = 4
		maxInflight    = 16
	)
	procs := runtime.GOMAXPROCS(0)
	send := int64(N)
	var wg sync.WaitGroup
	wg.Add(procs)
	for p := 0; p < procs; p++ {
		client, err := rpc.Dial("tcp", rpcServerAddr)
		if err != nil {
			log.Fatal("error dialing:", err)
		}
		var clientwg sync.WaitGroup
		clientwg.Add(clientsPerConn)
		go func() {
			clientwg.Wait()
			client.Close()
			wg.Done()
		}()
		for c := 0; c < clientsPerConn; c++ {
			resc := make(chan *rpc.Call, maxInflight+1)
			gate := make(chan struct{}, maxInflight)
			go func() {
				for atomic.AddInt64(&send, -1) >= 0 {
					gate <- struct{}{}
					req := &FindReq{"foo", 3}
					res := &FindRes{Start: time.Now()}
					client.Go("Server.Find", req, res, resc)
				}
				close(gate)
			}()
			go func() {
				defer clientwg.Done()
				for _ = range gate {
					call := <-resc
					if call.Error != nil {
						log.Fatalf("rpc failed: %v", call.Error)
					}
					res := call.Reply.(*FindRes)
					if len(res.Matches) != 3 {
						log.Fatalf("incorrect reply: %v", res)
					}
					driver.LatencyNote(res.Start)
				}
			}()
		}
	}
	wg.Wait()
}

type Server struct{}

func (s *Server) Find(req *FindReq, res *FindRes) error {
	res.Matches = []string{"aaa", "bbb", "ccc"}
	return nil
}

type FindReq struct {
	Query string
	N int
}

type FindRes struct {
	Matches []string
	Start   time.Time
}
