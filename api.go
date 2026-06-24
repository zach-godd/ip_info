package main

import (
	"encoding/json"
	"net/http"
	"time"
)

// LookupRequest is the JSON body accepted by POST /lookup.
type LookupRequest struct {
	URLs []string `json:"urls"`
}

func handleLookup(concurrency int, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req LookupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.URLs) == 0 {
			http.Error(w, `"urls" must be a non-empty array`, http.StatusBadRequest)
			return
		}

		results := runLookup(req.URLs, concurrency, timeout)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(results)
	}
}

func startServer(addr string, concurrency int, timeout time.Duration) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/lookup", handleLookup(concurrency, timeout))
	return http.ListenAndServe(addr, mux)
}
