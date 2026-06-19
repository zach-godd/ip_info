package main

import (
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
