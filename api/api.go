package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"main.go/network"
	"main.go/types"
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

func StartServer(addr string, concurrency int, timeout time.Duration) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/lookup", handleLookup(concurrency, timeout))
	return http.ListenAndServe(addr, mux)
}

// extractDomain extracts the domain from a URL
func extractDomain(urlStr string) (string, error) {
	if !strings.Contains(urlStr, "://") {
		urlStr = "http://" + urlStr
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("no hostname found in URL")
	}

	return host, nil
}

// processURLs processes a slice of URLs and gathers DNS data for each
func processURLs(urls []string, concurrency int, timeout time.Duration) []types.DNSData {
	results := make([]types.DNSData, 0, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup

	semaphore := make(chan struct{}, concurrency)

	for _, urlStr := range urls {
		wg.Add(1)
		go func(originalURL string) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			domain, err := extractDomain(originalURL)
			if err != nil {
				mu.Lock()
				results = append(results, types.DNSData{
					URL:       originalURL,
					Error:     err,
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				})
				mu.Unlock()
				return
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			data := network.GetDNSData(ctx, originalURL, domain)

			mu.Lock()
			results = append(results, data)
			mu.Unlock()
		}(urlStr)
	}

	wg.Wait()
	return results
}

// runLookup processes URLs and enriches each result with DMARC data.
func runLookup(urls []string, concurrency int, timeout time.Duration) []types.DNSData {
	results := processURLs(urls, concurrency, timeout)
	for i, result := range results {
		data, err := network.GetDMARCData(result.Domain)
		if err != nil {
			results[i].DMARCErr = err
		}
		results[i].DMARCRecord = data
	}
	return results
}
