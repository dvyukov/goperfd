package main

import (
	"log"
	"os"
	"net/http"

	"code.google.com/p/goperfd/config"
	"code.google.com/p/goperfd/builder"
	"code.google.com/p/goperfd/ui"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatalf("usage: %v project.cfg host.cfg", os.Args[0])
	}
	if err := config.Load(&config.Project, os.Args[1]); err != nil {
		log.Fatalf("failed to load project config file '%v' (%v)", os.Args[1], err)
	}
	if err := config.Load(&config.Host, os.Args[2]); err != nil {
		log.Fatalf("failed to load host config file '%v' (%v)", os.Args[2], err)
	}
	if err := ui.RegisterHandlers(); err != nil {
		log.Fatalf("failed to register ui handlers (%v)", err)
	}
	if err := builder.RegisterHandlers(); err != nil {
		log.Fatalf("failed to register builder handlers (%v)", err)
	}
	if err := http.ListenAndServe(config.Host.Addr, nil); err != nil {
		log.Fatalf("failed to listen and serve on '%v' (%v)", config.Host.Addr, err)
	}
}
