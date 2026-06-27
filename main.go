package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"main.go/api"
)

// TODO(ZSG) - Add more DNS information (e.g.  DKIM, etc.) and support for IPv6 addresses
// TODO(ZSG) - Add retries for transient errors and rate limiting handling for ip-api.com

func main() {
	addr := flag.String("addr", ":8080", "Address to listen on for HTTP API")
	concurrency := flag.Int("concurrency", 5, "Number of concurrent DNS lookups")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout for each DNS lookup")
	flag.Parse()

	fmt.Fprintf(os.Stderr, "Starting HTTP server on %s\n", *addr)
	apiServer := api.NewAPI()
	if err := apiServer.StartServer(*addr, *concurrency, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
