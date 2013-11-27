// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"log"
	"net"
	"net/rpc"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func init() {
	RegisterBenchmark("rpc", BenchmarkRPC)
}

var (
	rpcServerAddr string
	rpcResponse   FindRes
)

func BenchmarkRPC() PerfResult {
	if rpcServerAddr == "" {
		rpc.Register(new(Server))
		rpc.RegisterName("Server", new(Server))

		var buildResponse func(n *JSONNode)
		buildResponse = func(n *JSONNode) {
			rpcResponse.Matches = append(rpcResponse.Matches, *n)
			for i := 0; i < len(n.Kids) && len(rpcResponse.Matches) < 5; i++ {
				buildResponse(n.Kids[i])
			}
		}
		buildResponse(jsondata.Tree)

		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Fatalf("net.Listen tcp :0: %v", err)
		}
		rpcServerAddr = l.Addr().String()
		go rpc.Accept(l)
	}

	return PerfBenchmark(benchmarkRPC)
}

func benchmarkRPC(N uint64) {
	client, err := rpc.Dial("tcp", rpcServerAddr)
	if err != nil {
		log.Fatal("error dialing:", err)
	}
	defer client.Close()

	req := &FindReq{[]string{"aaaaa", "bbbbbbb", "ccccccc", "ddddddd", "eeeee"}}
	send := int64(N)
	recv := int64(N)
	procs := 4 * runtime.GOMAXPROCS(0)
	conc := 16 * procs
	gate := make(chan struct{}, conc)
	resc := make(chan *rpc.Call, conc)
	var wg sync.WaitGroup
	wg.Add(procs)
	for p := 0; p < procs; p++ {
		go func() {
			for atomic.AddInt64(&send, -1) >= 0 {
				gate <- struct{}{}
				res := &FindRes{Start: time.Now()}
				client.Go("Server.Find", req, res, resc)
			}
		}()
		go func() {
			for call := range resc {
				res := call.Reply.(*FindRes)
				if len(res.Matches) != 5 {
					log.Fatalf("incorrect reply: %v", res)
				}
				PerfLatencyNote(res.Start)
				<-gate
				if atomic.AddInt64(&recv, -1) == 0 {
					close(resc)
				}
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

type FindReq struct {
	Names []string
}

type FindRes struct {
	Matches []JSONNode
	Start   time.Time
}

type Server struct{}

func (s *Server) Find(req *FindReq, res *FindRes) error {
	*res = rpcResponse
	return nil
}
