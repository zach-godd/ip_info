package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// TODO(ZSG) - Add more DNS information (e.g. DMARC, DKIM, etc.) and support for IPv6 addresses
// TODO(ZSG) - Add retries for transient errors and rate limiting handling for ip-api.com
// TODO(ZSG) - Add unit tests

// DNSData holds all DNS information for a domain
type DNSData struct {
	URL        string     `json:"url"`
	Domain     string     `json:"domain"`
	MXRecords  []MXRecord `json:"mx_records,omitempty"`
	MXErr      string     `json:"mx_record_error,omitempty"`
	MxARecords []string   `json:"mx_a_records,omitempty"`
	SPFRecord  string     `json:"spf_record,omitempty"`
	MXASN      string     `json:"mxasn,omitempty"`
	ASNOrg     string     `json:"asn_org,omitempty"`
	ARecordErr string     `json:"a_record_error,omitempty"`
	Timestamp  string     `json:"timestamp"`
	Error      string     `json:"error,omitempty"`
}

// MXRecord represents an MX record
type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

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

// getASN retrieves the ASN information for an IP address using ip-api.com
func getASN(ctx context.Context, ipAddress string) (string, string) {
	urlStr := fmt.Sprintf("http://ip-api.com/json/%s?fields=as,org", ipAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", ""
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", ""
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", ""
	}

	asn := ""
	org := ""

	if as, ok := result["as"].(string); ok {
		asn = as
	}
	if organization, ok := result["org"].(string); ok {
		org = organization
	}

	return asn, org
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

// getDNSData retrieves all DNS data for a domain
func getDNSData(ctx context.Context, originalURL string, domain string) DNSData {
	data := DNSData{
		URL:       originalURL,
		Domain:    domain,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Create a resolver
	resolver := &net.Resolver{PreferGo: false}

	// Get MX records
	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		if !strings.Contains(err.Error(), "no such host") {
			data.MXErr = fmt.Sprintf("MX lookup failed: %v", err)
		}
	} else {
		for _, mx := range mxRecords {
			data.MXRecords = append(data.MXRecords, MXRecord{
				Host:     strings.TrimSuffix(mx.Host, "."),
				Priority: mx.Pref,
			})
		}
	}

	// Resolve first MX host to A records and, if available, get ASN for the first IP.
	if len(mxRecords) > 0 {
		firstMXHost := strings.TrimSuffix(mxRecords[0].Host, ".")
		addresses, err := resolver.LookupHost(ctx, firstMXHost)
		if err != nil {
			if data.ARecordErr == "" {
				data.ARecordErr = fmt.Sprintf("A record lookup failed: %v", err)
			}
		} else {
			data.MxARecords = addresses
			if len(addresses) > 0 {
				asn, org := getASN(ctx, addresses[0])
				data.MXASN = asn
				data.ASNOrg = org
			}
		}
	}

	// Get SPF records
	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err == nil {
		for _, txt := range txtRecords {
			if strings.HasPrefix(txt, "v=spf1") {
				data.SPFRecord = txt
				break
			}
		}
	}

	return data
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
					Error:     err.Error(),
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

func getDMARCData(domain string) (string, error) {
	dmarcDomain := "_dmarc." + domain

	txtRecords, err := net.DefaultResolver.LookupTXT(context.Background(), dmarcDomain)
	if err != nil {
		fmt.Printf("Failed to look up DMARC record: %v\n", err)
		return "", err
	}

	for _, record := range txtRecords {
		if strings.HasPrefix(record, "v=DMARC1") {
			return record, nil
		}
	}
	return "", errors.New("no DMARC records found")
}
