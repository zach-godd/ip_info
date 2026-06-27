package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type APITestSuite struct {
	suite.Suite
	api API
}

func (t *APITestSuite) SetupTest() {
	t.api = NewAPI()
}

func APITestSuitRun(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}

func (t *APITestSuite) TestHandleLookup() {
	handler := t.api.handleLookup(5, 10*time.Second)

	tests := map[string]struct {
		method     string
		body       []byte
		wantStatus int
		wantCount  int
	}{
		"valid lookup": {
			method:     http.MethodPost,
			body:       []byte(`{"urls":["google.com"]}`),
			wantStatus: http.StatusOK,
			wantCount:  1,
		},
		"wrong method": {
			method:     http.MethodGet,
			wantStatus: http.StatusMethodNotAllowed,
		},
		"empty urls": {
			method:     http.MethodPost,
			body:       []byte(`{"urls":[]}`),
			wantStatus: http.StatusBadRequest,
		},
		"invalid body": {
			method:     http.MethodPost,
			body:       []byte(`not valid json`),
			wantStatus: http.StatusBadRequest,
		},
	}

	for name, tc := range tests {
		t.Run(name, func() {
			req := httptest.NewRequest(tc.method, "/lookup", bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler(w, req)

			t.Assert().Equal(tc.wantStatus, w.Code)

			if tc.wantCount > 0 {
				var results []map[string]any
				t.Require().NoError(json.NewDecoder(w.Body).Decode(&results))
				t.Assert().Len(results, tc.wantCount)
			}
		})
	}
}

func (t *APITestSuite) TestExtractDomain() {
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
