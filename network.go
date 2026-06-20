package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

var badStatus error = errors.New("non-200 status code")

// TODO (ZSG) - Add tests for getDNS
// TODO (ZSG) - Add mocks for more robust testing

type ASNInfo struct {
	ASN          string `json:"asn"`
	Organization string `json:"organization"`
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
				asn, err := getASN(ctx, addresses[0])
				if err != nil {
					data.ASNErr = err.Error()
				} else {
					data.MXASN = *asn
				}
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

// getDMARCData gets dmarc data about a domain using a LookupTXT call
func getDMARCData(domain string) (string, error) {
	dmarcDomain := "_dmarc." + domain

	r := net.Resolver{PreferGo: false}
	txtRecords, err := r.LookupTXT(context.Background(), dmarcDomain)
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

// getASN retrieves the ASN information for an IP address using ip-api.com
func getASN(ctx context.Context, ipAddress string) (*ASNInfo, error) {
	urlStr := fmt.Sprintf("http://ip-api.com/json/%s?fields=as,org", ipAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, badStatus
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	var asnInfo ASNInfo

	if as, ok := result["as"].(string); ok {
		asnInfo.ASN = as
	}
	if organization, ok := result["org"].(string); ok {
		asnInfo.Organization = organization
	}

	return &asnInfo, nil
}
