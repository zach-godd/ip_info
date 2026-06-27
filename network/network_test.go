package network

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/suite"
)

type NetworkTestSuite struct {
	retriever DataRetriever
	suite.Suite
}

func TestNetworkTestSuite(t *testing.T) {
	suite.Run(t, new(NetworkTestSuite))
}

func (t *NetworkTestSuite) SetupTest() {
	t.retriever = NewDataRetriever(&net.Resolver{PreferGo: false})
}

func (t *NetworkTestSuite) TestGetDMARC() {
	tests := map[string]struct {
		domain string
		err    error
	}{
		"valid": {
			domain: "google.com",
		},
		"invalid domain": {
			domain: "thisdomaintotallydoesnotexist.com",
			err:    errors.New("lookup _dmarc.thisdomaintotallydoesnotexist.com: no such host"),
		},
	}
	for name, test := range tests {
		t.Run(name, func() {
			result, err := t.retriever.GetDMARCData(test.domain)
			if test.err != nil {
				t.Require().EqualError(err, test.err.Error())
			} else {
				t.Assert().NoError(err)
				t.Assert().NotNil(result)
			}
		})
	}
}

func (t *NetworkTestSuite) TestGetASN() {
	tests := map[string]struct {
		domain string
	}{
		"valid": {
			domain: "google.com",
		},
	}

	for label, test := range tests {
		t.Run(label, func() {
			asn, _ := getASN(context.Background(), test.domain)
			t.Assert().NotNil(asn.ASN)
			t.Assert().NotNil(asn.Organization)
		})
	}
}

func (t *NetworkTestSuite) TestGetSPF() {
	tests := map[string]struct {
		domain      string
		expectedErr error
	}{
		"valid": {
			domain: "google.com",
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
			t.Require().NotNil(spf)
		})
	}
}

func (t *NetworkTestSuite) TestGetARecord() {
	tests := map[string]struct {
		domain      string
		expectedErr error
	}{
		"valid": {
			domain: "google.com",
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
			t.Require().NotNil(spf)
		})
	}
}
