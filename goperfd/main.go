package main

import (
	"log"
	"os"

	"code.google.com/p/goperfd/config"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %v project.cfg machine.cfg", os.Args[0])
	}
	if err := config.Load(&config.Project, os.Args[1]); err != nil {
		log.Fatalf("failed to load project config file '%v' (%v)", os.Args[1], err)
	}
	if err := config.Load(&config.Machine, os.Args[2]); err != nil {
		log.Fatalf("failed to load machine config file '%v' (%v)", os.Args[2], err)
	}
}
