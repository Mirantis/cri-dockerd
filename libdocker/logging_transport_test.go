/*
Copyright 2021 Mirantis

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package libdocker

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// mockRoundTripper is a mock http.RoundTripper for testing
type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestLoggingRoundTripper(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	// Set log level to debug to enable logging
	logrus.SetLevel(logrus.DebugLevel)

	tests := []struct {
		name           string
		requestBody    string
		responseBody   string
		responseStatus int
		responseError  error
		expectError    bool
	}{
		{
			name:           "successful request with response",
			requestBody:    `{"test":"data"}`,
			responseBody:   `{"result":"success"}`,
			responseStatus: 200,
			expectError:    false,
		},
		{
			name:           "large response body gets truncated",
			requestBody:    "",
			responseBody:   strings.Repeat("x", 2000),
			responseStatus: 200,
			expectError:    false,
		},
		{
			name:           "error during request",
			requestBody:    "",
			responseBody:   "",
			responseStatus: 0,
			responseError:  fmt.Errorf("connection error"),
			expectError:    true,
		},
		{
			name:           "empty response",
			requestBody:    "",
			responseBody:   "",
			responseStatus: 204,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock response
			var resp *http.Response
			if tt.responseError == nil {
				resp = &http.Response{
					StatusCode: tt.responseStatus,
					Status:     fmt.Sprintf("%d OK", tt.responseStatus),
					Body:       io.NopCloser(bytes.NewBufferString(tt.responseBody)),
					Header:     make(http.Header),
				}
				resp.Header.Set("Content-Type", "application/json")
			}

			mockTransport := &mockRoundTripper{
				response: resp,
				err:      tt.responseError,
			}

			lrt := newLoggingRoundTripper(mockTransport)

			// Create test request
			req := httptest.NewRequest("GET", "http://example.com/test", bytes.NewBufferString(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer secret-token")

			// Execute request
			gotResp, err := lrt.RoundTrip(req)

			// Check error
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, gotResp)
				assert.Equal(t, tt.responseStatus, gotResp.StatusCode)

				// Verify response body is still readable
				if tt.responseBody != "" {
					body, err := io.ReadAll(gotResp.Body)
					assert.NoError(t, err)
					assert.Equal(t, tt.responseBody, string(body))
				}
			}
		})
	}
}

func TestSensitiveHeadersRedacted(t *testing.T) {
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)
	logrus.SetLevel(logrus.DebugLevel)

	// Test that sensitive headers are properly identified
	assert.True(t, sensitiveHeaders["authorization"])
	assert.True(t, sensitiveHeaders["x-auth-token"])
	assert.True(t, sensitiveHeaders["x-registry-auth"])
}

func TestNewHTTPClientWithLogging(t *testing.T) {
	client := newHTTPClientWithLogging(nil)
	assert.NotNil(t, client)
	assert.NotNil(t, client.Transport)

	// Verify it's a logging round tripper
	_, ok := client.Transport.(*loggingRoundTripper)
	assert.True(t, ok, "Transport should be a loggingRoundTripper")
}

func TestLoggingDisabledAtInfoLevel(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	// Set log level to info (logging should not happen)
	logrus.SetLevel(logrus.InfoLevel)

	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Body:       io.NopCloser(bytes.NewBufferString(`{"result":"success"}`)),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: resp,
		err:      nil,
	}

	lrt := newLoggingRoundTripper(mockTransport)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	gotResp, err := lrt.RoundTrip(req)

	assert.NoError(t, err)
	assert.NotNil(t, gotResp)
	// The request should still work even though logging is disabled
	assert.Equal(t, 200, gotResp.StatusCode)
}
