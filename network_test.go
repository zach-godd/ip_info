package main

import (
	"context"
	"errors"
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
			asn, org := getASN(context.Background(), test.domain)
			t.Assert().Equal(test.expectedASN, asn)
			t.Assert().Equal(test.expectedOrg, org)
		})
	}
}
