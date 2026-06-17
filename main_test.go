package main

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type TestSuite struct {
	suite.Suite
}

func TestSuitRun(t *testing.T) {
	suite.Run(t, new(TestSuite))
}
func (t *TestSuite) TestExtractDomain() {
	tests := map[string]struct {
		url            string
		expectedDomain string
	}{
		"valid": {
			url:            "http://example.com",
			expectedDomain: "example.com",
		},
	}

	for name, test := range tests {
		t.Run(name, func() {
			domain, err := extractDomain(test.url)
			t.Require().NoError(err)
			t.Assert().Equal(test.expectedDomain, domain)
		})
	}
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
