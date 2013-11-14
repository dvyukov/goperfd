package config

import (
	"encoding/json"
	"os"
	"io/ioutil"
)

type MachineConfig struct {
	Addr string
}

type ProjectConfig struct {
	Name string
}

var Machine MachineConfig
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
