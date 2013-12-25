// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package widefinder

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"code.google.com/p/goperfd/bench/driver"
)

func init() {
	driver.Register("widefinder", benchmark)
}

func benchmark() driver.Result {
	return driver.Benchmark(benchmarkN)
}

type LogEntry struct {
	Status  int
	Bytes   int
	URL     string
	Client  string
	Referer string
}

const (
	LogHits = iota
	LogBytes
	LogMisses
	LogClients
	LogReferers
	LogStatCount
)

type LogResult struct {
	Stats [LogStatCount]map[string]int
}

var (
	reArticle     = regexp.MustCompile("^/ongoing/When/[0-9]{3}x/[0-9]{4}/[0-9]{2}/[0-9]{2}/[^ .]+$")
	reInternalRef = regexp.MustCompile("^http://www.tbray.org/ongoing/")

	printResults = flag.Bool("printresults", false, "print widefinder results")
)

func benchmarkN(N uint64) {
	procs := runtime.GOMAXPROCS(0)
	chancap := 16 * procs

	// Read the file and stream individual lines to linec.
	linec := make(chan string, chancap)
	go readAndFeed(N, linec)

	// Read lines from linec, parse and stream LogEntry's to entryc.
	entryc := make(chan *LogEntry, chancap)
	var entrywg sync.WaitGroup
	entrywg.Add(procs)
	for i := 0; i < procs; i++ {
		go func() {
			for line := range linec {
				if e := parseLogLine(line); e != nil {
					entryc <- e
				}
			}
			entrywg.Done()
		}()
	}
	go func() {
		entrywg.Wait()
		close(entryc)
	}()

	// Read LogEntry's from entryc and process them; send results to resc at the end.
	resc := make(chan *LogResult, procs)
	var reswg sync.WaitGroup
	reswg.Add(procs)
	for i := 0; i < procs; i++ {
		go func() {
			res := &LogResult{}
			for i := range res.Stats {
				res.Stats[i] = make(map[string]int)
			}
			for e := range entryc {
				processLogEntry(res, e)
			}
			resc <- res
			reswg.Done()
		}()
	}
	go func() {
		reswg.Wait()
		close(resc)
	}()

	// Collect all partial results.
	var results []*LogResult
	for res := range resc {
		results = append(results, res)
	}

	var stats [LogStatCount]LogStats
	var wg sync.WaitGroup
	wg.Add(LogStatCount)
	for i := 0; i < LogStatCount; i++ {
		go func(i int) {
			stats[i] = aggregateResults(results, i)
			wg.Done()
		}(i)
	}
	wg.Wait()

	reportResults(stats[LogHits], "URIs by hit", false)
	reportResults(stats[LogBytes], "URIs by bytes", true)
	reportResults(stats[LogMisses], "404s", false)
	reportResults(stats[LogClients], "client addresses", false)
	reportResults(stats[LogReferers], "referers", false)
}

func readAndFeed(N uint64, linec chan string) {
	filename := filepath.Join("widefinder", "widefinder.log")
	f, err := os.Open(filename)
	if os.IsNotExist(err) {
		filename = filepath.Join(driver.WorkDir, "..", "gopath", "src", "code.google.com", "p", "goperfd", "bench", filename)
		f, err = os.Open(filename)
	}
	if err != nil {
		log.Fatalf("failed to open %v: %v", filename, err)
	}
	defer f.Close()
	for N > 0 {
		f.Seek(0, 0)
		r := bufio.NewReader(f)
		for ; N > 0; N-- {
			ln, err := r.ReadString('\n')
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatalf("scanner failed: %v", err)
			}
			linec <- ln
		}
	}
	close(linec)
}

func parseLogLine(line string) *LogEntry {
	ss := strings.Split(line, " ")
	if len(ss) < 11 {
		return nil
	}
	if ss[5] != "\"GET" {
		return nil
	}
	status, err := strconv.Atoi(ss[8])
	if err != nil {
		return nil
	}
	bytes := 0
	if status == 200 {
		bytes, err = strconv.Atoi(ss[9])
		if err != nil {
			return nil
		}
	}
	referer := ss[10][1 : len(ss[10])-1]
	return &LogEntry{status, bytes, ss[6], ss[0], referer}

}

func processLogEntry(res *LogResult, e *LogEntry) {
	switch e.Status {
	case 200, 304:
		if e.Bytes != 0 {
			res.Stats[LogBytes][e.URL] = res.Stats[LogBytes][e.URL] + e.Bytes
		}
		if reArticle.MatchString(e.URL) {
			res.Stats[LogHits][e.URL] = res.Stats[LogHits][e.URL] + 1
			res.Stats[LogClients][e.Client] = res.Stats[LogClients][e.Client] + 1
			if !reInternalRef.MatchString(e.Referer) {
				res.Stats[LogReferers][e.Referer] = res.Stats[LogReferers][e.Referer] + 1
			}
		}
	case 404:
		res.Stats[LogMisses][e.URL] = res.Stats[LogMisses][e.URL] + 1
	}
}

type LogStat struct {
	Str string
	Val int
}

type LogStats []LogStat

func (p LogStats) Len() int           { return len(p) }
func (p LogStats) Less(i, j int) bool { return p[i].Val > p[j].Val }
func (p LogStats) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

func aggregateResults(results []*LogResult, stat int) LogStats {
	res := make(map[string]int)
	for _, res1 := range results {
		for k, v := range res1.Stats[stat] {
			res[k] = res[k] + v
		}
	}
	var stats LogStats
	for k, v := range res {
		stats = append(stats, LogStat{k, v})
	}
	sort.Sort(stats)
	return stats
}

func reportResults(stats LogStats, caption string, shrink bool) {
	if !*printResults {
		return
	}
	fmt.Printf("Top %s:\n", caption)
	for i := 0; i < len(stats) && i < 10; i++ {
		s := stats[i]
		if shrink {
			fmt.Printf(" %9.1fM: %s\n", float64(s.Val)/(1<<20), s.Str)
		} else {
			fmt.Printf(" %10d: %s\n", s.Val, s.Str)
		}
	}
	fmt.Printf("\n")
}
