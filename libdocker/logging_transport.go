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
	"io"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	// Maximum size of response body to log (1KB)
	maxResponseBodyLogSize = 1024
)

// sensitiveHeaders contains headers that should be redacted in logs
var sensitiveHeaders = map[string]bool{
	"authorization": true,
	"x-auth-token":  true,
	"x-registry-auth": true,
}

// loggingRoundTripper is an http.RoundTripper that logs HTTP requests and responses
type loggingRoundTripper struct {
	transport http.RoundTripper
}

// newLoggingRoundTripper creates a new logging round tripper
func newLoggingRoundTripper(transport http.RoundTripper) http.RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &loggingRoundTripper{
		transport: transport,
	}
}

// RoundTrip executes a single HTTP transaction and logs the request and response
func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Log the request
	logrus.Debugf("Docker HTTP Request: %s %s", req.Method, req.URL.String())
	
	// Log request headers (with sensitive data redacted)
	if logrus.GetLevel() >= logrus.DebugLevel {
		for key, values := range req.Header {
			lowerKey := strings.ToLower(key)
			if sensitiveHeaders[lowerKey] {
				logrus.Debugf("  %s: [REDACTED]", key)
			} else {
				logrus.Debugf("  %s: %s", key, strings.Join(values, ", "))
			}
		}
	}

	// Log request body for non-streaming requests (if present and small enough)
	if req.Body != nil && req.ContentLength > 0 && req.ContentLength < maxResponseBodyLogSize {
		bodyBytes, err := io.ReadAll(req.Body)
		if err == nil {
			// Restore the body for the actual request
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			logrus.Debugf("  Request Body: %s", string(bodyBytes))
		}
	}

	// Execute the actual request
	resp, err := lrt.transport.RoundTrip(req)
	
	// Log errors
	if err != nil {
		logrus.Debugf("Docker HTTP Request Error: %v", err)
		return resp, err
	}

	// Log the response
	logrus.Debugf("Docker HTTP Response: %d %s", resp.StatusCode, resp.Status)
	
	// Log response headers
	if logrus.GetLevel() >= logrus.DebugLevel {
		for key, values := range resp.Header {
			logrus.Debugf("  %s: %s", key, strings.Join(values, ", "))
		}
	}

	// Log response body (truncated if too large)
	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			// Restore the body for the caller
			resp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			
			// Log the body (truncated if necessary)
			if len(bodyBytes) > 0 {
				logSize := len(bodyBytes)
				if logSize > maxResponseBodyLogSize {
					logrus.Debugf("  Response Body (%d bytes, truncated to %d): %s...", 
						len(bodyBytes), maxResponseBodyLogSize, string(bodyBytes[:maxResponseBodyLogSize]))
				} else {
					logrus.Debugf("  Response Body (%d bytes): %s", len(bodyBytes), string(bodyBytes))
				}
			}
		} else {
			logrus.Debugf("  Error reading response body: %v", err)
		}
	}

	return resp, nil
}

// newHTTPClientWithLogging creates an HTTP client with logging transport
func newHTTPClientWithLogging(baseTransport http.RoundTripper) *http.Client {
	return &http.Client{
		Transport: newLoggingRoundTripper(baseTransport),
	}
}
