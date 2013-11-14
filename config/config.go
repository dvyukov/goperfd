package config

import (
	"encoding/json"
	"io/ioutil"
	"os"
)

type HostConfig struct {
	Addr string
}

type ProjectConfig struct {
	Name       string
	Benchmarks []Benchmark
	Machines   []Machine
	Metrics    []Metric
}

type Benchmark struct {
	Name string
	Desc string
}

type Machine struct {
	Name string
	Desc string
	Key  string
}

type Metric struct {
	Name string
	Desc string
}

var Host HostConfig
var Project ProjectConfig

func Load(cfg interface{}, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	buf, err := ioutil.ReadAll(f)
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf, cfg)
	if err != nil {
		return err
	}
	return nil
}
