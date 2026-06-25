package types

type ASNInfo struct {
	ASN          string `json:"asn"`
	Organization string `json:"organization"`
}

type MXRecord struct {
	Host     string `json:"host"`
	Priority uint16 `json:"priority"`
}

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
