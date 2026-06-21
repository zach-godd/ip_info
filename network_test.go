package main

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
)

type NetworkTestSuite struct {
	suite.Suite
}

func TestNetworkTestSuite(t *testing.T) {
	suite.Run(t, new(NetworkTestSuite))
}

func (t *TestSuite) TestGetDMARC() {
	tests := map[string]struct {
		domain         string
		expectedResult string
		err            error
	}{
		"valid": {
			domain:         "google.com",
			expectedResult: "v=DMARC1; p=reject; rua=mailto:mailauth-reports@google.com",
		},
		"invalid domain": {
			domain: "thisdomaintotallydoesnotexist.com",
			err:    errors.New("lookup _dmarc.thisdomaintotallydoesnotexist.com: no such host"),
		},
	}
	for name, test := range tests {
		t.Run(name, func() {
			result, err := getDMARCData(test.domain)
			if test.err != nil {
				t.Require().EqualError(err, test.err.Error())
			} else {
				t.Assert().NoError(err)
			}

			t.Assert().Equal(test.expectedResult, result)
		})
	}
}

func (t *TestSuite) TestGetASN() {
	tests := map[string]struct {
		domain      string
		expectedASN string
		expectedOrg string
	}{
		"valid": {
			domain:      "google.com",
			expectedASN: "AS15169 Google LLC",
			expectedOrg: "Google LLC",
		},
		"invalid domain": {
			domain: "thisdomaintotallydoesnotexist.com",
		},
	}

	for label, test := range tests {
		t.Run(label, func() {
			asn, _ := getASN(context.Background(), test.domain)
			t.Assert().Equal(test.expectedASN, asn.ASN)
			t.Assert().Equal(test.expectedOrg, asn.Organization)
		})
	}
}

func (t *TestSuite) TestGetSPF() {
	tests := map[string]struct {
		domain      string
		spfRecord   string
		expectedErr error
	}{
		"valid": {
			domain:    "google.com",
			spfRecord: "v=spf1 include:_spf.google.com ~all",
		},
		"invalid domain": {
			domain:      "thisdomaintotallydoesnotexist.com",
			expectedErr: notFoundErr,
		},
	}
	for label, test := range tests {
		t.Run(label, func() {
			spf, err := getSPFRecords(context.Background(), test.domain)
			if test.expectedErr != nil {
				t.Require().EqualError(err, test.expectedErr.Error())
			} else {
				t.Assert().NoError(err)
			}
			t.Require().Equal(test.spfRecord, spf)
		})
	}
}

func (t *TestSuite) TestGetARecord() {
	tests := map[string]struct {
		domain      string
		ARecords    []string
		expectedErr error
	}{
		"valid": {
			domain:   "google.com",
			ARecords: []string{"2607:f8b0:4002:c08::1a", "2607:f8b0:4002:c08::1b", "2607:f8b0:4002:c05::1b", "2607:f8b0:4002:c05::1a", "172.253.124.26", "74.125.136.26", "74.125.136.27", "142.250.105.26", "172.253.124.27"},
		},
	}
	for label, test := range tests {
		t.Run(label, func() {
			r := net.Resolver{PreferGo: false}
			mxRecords, err := r.LookupMX(context.Background(), test.domain)

			spf, err := getARecords(context.Background(), mxRecords)
			if test.expectedErr != nil {
				t.Require().EqualError(err, test.expectedErr.Error())
			} else {
				t.Assert().NoError(err)
			}
			t.Require().Equal(test.ARecords, spf)
		})
	}
}
