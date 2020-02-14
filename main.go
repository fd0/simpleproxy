package main

import (
	"log"
	"net/http"
	"os"
)

var logger = log.New(os.Stderr, "simpleproxy: ", log.LstdFlags)

const Listen = "127.0.0.1:8080"

func main() {
	var addr = Listen
	if len(os.Args) > 1 {
		addr = os.Args[1]
	}

	logger.Printf("listen on %v", addr)

	// disable compression handling
	t := http.DefaultTransport.(*http.Transport)
	t.DisableCompression = true

	srv := &http.Server{
		Addr:     addr,
		ErrorLog: logger,
		Handler:  &proxy{client: http.DefaultClient},
	}

	logger.Fatal(srv.ListenAndServe())
}
