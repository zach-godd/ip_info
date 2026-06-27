package network

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

	"main.go/types"
)

var badStatus = errors.New("non-200 status code")
var notFoundErr = errors.New("not found")

// TODO (ZSG) - Remove tests at this level;
// They are integration tests that break easy. Move up to API package and test with mockery

type DataRetriever struct {
	resolver net.Resolver
}

func NewDataRetriever(resolver *net.Resolver) DataRetriever {
	return DataRetriever{
		resolver: *resolver,
	}
}

// GetDNSData retrieves all DNS data for a domain
func (d *DataRetriever) GetDNSData(ctx context.Context, originalURL string, domain string) types.DNSData {
	data := types.DNSData{
		URL:       originalURL,
		Domain:    domain,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Get MX records
	mxRecords, err := d.resolver.LookupMX(ctx, domain)
	if err != nil {
		if !strings.Contains(err.Error(), "no such host") {
			data.MXErr = err
		}
	} else {
		for _, mx := range mxRecords {
			data.MXRecords = append(data.MXRecords, types.MXRecord{
				Host:     strings.TrimSuffix(mx.Host, "."),
				Priority: mx.Pref,
			})
		}
	}

	// Resolve first MX host to A records and, if available, get ASN for the first IP.
	data.MxARecords, data.ARecordErr = getARecords(ctx, mxRecords)
	if data.MxARecords != nil && len(data.MxARecords) > 0 {
		asn, err := getASN(ctx, data.MxARecords[0])
		if err != nil {
			data.ARecordErr = err
		}
		data.MXASN = asn
	}

	// Get SPF records
	data.SPFRecord, data.SPFErr = getSPFRecords(ctx, data.MxARecords[0])

	return data
}

// GetDMARCData gets dmarc data about a domain using a LookupTXT call
func (d *DataRetriever) GetDMARCData(domain string) (string, error) {
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
func getASN(ctx context.Context, ipAddress string) (types.ASNInfo, error) {
	urlStr := fmt.Sprintf("http://ip-api.com/json/%s?fields=as,org", ipAddress)

	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return types.ASNInfo{}, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return types.ASNInfo{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return types.ASNInfo{}, badStatus
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.ASNInfo{}, err
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return types.ASNInfo{}, err
	}

	var asnInfo types.ASNInfo
	if as, ok := result["as"].(string); ok {
		asnInfo.ASN = as
	}
	if organization, ok := result["org"].(string); ok {
		asnInfo.Organization = organization
	}

	return asnInfo, nil
}

func getSPFRecords(ctx context.Context, domain string) (string, error) {
	resolver := &net.Resolver{PreferGo: false}
	txtRecords, err := resolver.LookupTXT(ctx, domain)
	if err == nil {
		for _, txt := range txtRecords {
			if strings.HasPrefix(txt, "v=spf1") {
				return txt, nil
			}
		}
	}
	return "", notFoundErr
}

func getARecords(ctx context.Context, mxRecords []*net.MX) ([]string, error) {
	resolver := &net.Resolver{PreferGo: false}
	firstMXHost := strings.TrimSuffix(mxRecords[0].Host, ".")
	addresses, err := resolver.LookupHost(ctx, firstMXHost)
	if err != nil {
		return nil, err
	}

	return addresses, nil
}
