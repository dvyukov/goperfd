package ui

import (
	"fmt"
	"net/http"
)

func RegisterHandlers() error {
	http.HandleFunc("/", handleRoot)
	return nil
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello!")
}
