package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// TODO(ZSG) - Add more DNS information (e.g.  DKIM, etc.) and support for IPv6 addresses
// TODO(ZSG) - Add retries for transient errors and rate limiting handling for ip-api.com
// TODO(ZSG) - Add unit tests

// DNSData holds all DNS information for a domain
type DNSData struct {
	URL         string     `json:"url"`
	Domain      string     `json:"domain"`
	MXRecords   []MXRecord `json:"mx_records,omitempty"`
	MXErr       error      `json:"mx_record_error,omitempty"`
	MxARecords  []string   `json:"mx_a_records,omitempty"`
	SPFRecord   string     `json:"spf_record,omitempty"`
	SPFErr      error      `json:"spf_record_error,omitempty"`
	DMARCRecord string     `json:"dmarc_records,omitempty"`
	DMARCErr    error      `json:"dmarc_record_error,omitempty"`
	MXASN       ASNInfo    `json:"mxasn,omitempty"`
	ASNErr      error      `json:"asn_err,omitempty"`
	ARecordErr  error      `json:"a_record_error,omitempty"`
	Timestamp   string     `json:"timestamp"`
	Error       error      `json:"error,omitempty"`
}

// MXRecord represents an MX record
type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

// TODO: Break into testable function calls
func main() {
	var (
		input       = flag.String("input", "example.txt", "Path to input file containing one domain/URL per line")
		output      = flag.String("output", "", "Output file for JSON results (default: stdout)")
		concurrency = flag.Int("concurrency", 5, "Number of concurrent DNS lookups")
		timeout     = flag.Duration("timeout", 10*time.Second, "Timeout for each DNS lookup")
	)

	flag.Parse()

	if input == nil {
		fmt.Fprint(os.Stderr, "Error: -input is required\n")
		flag.Usage()
		os.Exit(1)
	}

	urls, err := readLines(*input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input file: %v\n", err)
		os.Exit(1)
	}
	if len(urls) == 0 {
		fmt.Fprintf(os.Stderr, "No domains/URLs found in input file: %s\n", *input)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Processing %d URLs with %d concurrent workers, timeout: %v\n", len(urls), *concurrency, *timeout)

	// Process URLs
	results := processURLs(urls, *concurrency, *timeout)

	// add dmarc data
	for _, result := range results {
		data, err := getDMARCData(result.Domain)
		if err != nil {
			result.DMARCErr = err
		}
		result.DMARCRecord = data
	}

	// Convert to JSON
	jsonData, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		os.Exit(1)
	}

	// Output results
	if *output != "" {
		err = os.WriteFile(*output, jsonData, 0644)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
			os.Exit(1)
		}
		_, _ = fmt.Fprintf(os.Stderr, "Results written to %s\n", *output)
	} else {
		fmt.Println(string(jsonData))
	}
}

// readLines reads one domain/URL per line, skipping empty lines and comments.
func readLines(filePath string) ([]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return lines, nil
}

// extractDomain extracts the domain from a URL
func extractDomain(urlStr string) (string, error) {
	// Add scheme if missing
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
func processURLs(urls []string, concurrency int, timeout time.Duration) []DNSData {
	results := make([]DNSData, 0, len(urls))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, concurrency)

	for _, urlStr := range urls {
		wg.Add(1)
		go func(originalURL string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			domain, err := extractDomain(originalURL)
			if err != nil {
				mu.Lock()
				results = append(results, DNSData{
					URL:       originalURL,
					Error:     err,
					Timestamp: time.Now().UTC().Format(time.RFC3339),
				})
				mu.Unlock()
				return
			}

			// Create context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			data := getDNSData(ctx, originalURL, domain)

			mu.Lock()
			results = append(results, data)
			mu.Unlock()
		}(urlStr)
	}

	wg.Wait()
	return results
}
