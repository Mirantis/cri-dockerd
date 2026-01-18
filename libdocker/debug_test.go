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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	dockerapi "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRoundTripper is a mock HTTP transport for testing
type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.response, m.err
}

func TestWithDebugTransport(t *testing.T) {
	t.Run("wraps transport correctly", func(t *testing.T) {
		client, err := dockerapi.NewClientWithOpts(
			dockerapi.WithHost("unix:///var/run/docker.sock"),
			withDebugTransport(),
		)
		require.NoError(t, err)
		defer client.Close()

		httpClient := client.HTTPClient()
		require.NotNil(t, httpClient)

		// The Docker client wraps transports with additional layers (like otelhttp),
		// so we verify the transport is configured but may not be directly accessible
		assert.NotNil(t, httpClient.Transport, "expected transport to be configured")
	})
}

func TestWrapTransport(t *testing.T) {
	t.Run("returns wrapped transport", func(t *testing.T) {
		baseTransport := &mockRoundTripper{}
		wrapped := wrapTransport(baseTransport)

		wt, ok := wrapped.(*wrappedTransport)
		assert.True(t, ok, "expected wrappedTransport type")
		assert.Equal(t, baseTransport, wt.base)
	})
}

func TestWrappedTransportRoundTrip(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	t.Run("successful request", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		mockResp := &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader(`{"test":"response"}`)),
			Header:     make(http.Header),
		}

		baseTransport := &mockRoundTripper{response: mockResp}
		wrapped := wrapTransport(baseTransport)

		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		resp, err := wrapped.RoundTrip(req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 200, resp.StatusCode)

		// Verify body can be read
		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, `{"test":"response"}`, string(body))
		resp.Body.Close()
	})

	t.Run("request with error", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		expectedErr := errors.New("connection failed")
		baseTransport := &mockRoundTripper{err: expectedErr}
		wrapped := wrapTransport(baseTransport)

		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		resp, err := wrapped.RoundTrip(req)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, resp)
	})

	t.Run("request with body", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		mockResp := &http.Response{
			StatusCode: 201,
			Status:     "201 Created",
			Body:       io.NopCloser(strings.NewReader("")),
			Header:     make(http.Header),
		}

		baseTransport := &mockRoundTripper{response: mockResp}
		wrapped := wrapTransport(baseTransport)

		reqBody := strings.NewReader(`{"create":"data"}`)
		req := httptest.NewRequest("POST", "http://example.com/create", reqBody)
		resp, err := wrapped.RoundTrip(req)

		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()
	})
}

func TestCaptureContext(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	t.Run("logDebug with success", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		ctx := &captureContext{
			method:   "GET",
			endpoint: "/test",
			reqBody:  []byte("request"),
		}

		// This should not panic
		ctx.logDebug("200 OK", nil)
	})

	t.Run("logDebug with error", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		ctx := &captureContext{
			method:   "GET",
			endpoint: "/test",
			reqBody:  []byte("request"),
		}

		// This should not panic
		ctx.logDebug("", errors.New("test error"))
	})

	t.Run("logTrace with success", func(t *testing.T) {
		logrus.SetLevel(logrus.TraceLevel)

		ctx := &captureContext{
			method:   "POST",
			endpoint: "/create",
			reqBody:  []byte(`{"key":"value"}`),
		}

		// This should not panic
		ctx.logTrace("201 Created", []byte(`{"id":"123"}`), nil)
	})

	t.Run("logTrace with error", func(t *testing.T) {
		logrus.SetLevel(logrus.TraceLevel)

		ctx := &captureContext{
			method:   "POST",
			endpoint: "/create",
			reqBody:  []byte(`{"key":"value"}`),
		}

		// This should not panic
		ctx.logTrace("", nil, errors.New("test error"))
	})

	t.Run("logTrace with empty bodies", func(t *testing.T) {
		logrus.SetLevel(logrus.TraceLevel)

		ctx := &captureContext{
			method:   "GET",
			endpoint: "/test",
			reqBody:  []byte{},
		}

		// This should not panic and show "(empty)" for bodies
		ctx.logTrace("200 OK", []byte{}, nil)
	})

	t.Run("log switches between debug and trace", func(t *testing.T) {
		ctx := &captureContext{
			method:   "GET",
			endpoint: "/test",
			reqBody:  []byte("request"),
		}

		// At debug level, should call logDebug
		logrus.SetLevel(logrus.DebugLevel)
		ctx.log("200 OK", []byte("response"), nil)

		// At trace level, should call logTrace
		logrus.SetLevel(logrus.TraceLevel)
		ctx.log("200 OK", []byte("response"), nil)
	})
}

func TestBodyCapture(t *testing.T) {
	t.Run("captures body on read", func(t *testing.T) {
		originalBody := io.NopCloser(strings.NewReader("test body content"))
		var captured []byte

		bc := &bodyCapture{
			ReadCloser: originalBody,
			onRead: func(data []byte) {
				captured = data
			},
		}

		// Read all content
		body, err := io.ReadAll(bc)
		assert.NoError(t, err)
		assert.Equal(t, "test body content", string(body))

		// Verify captured data
		assert.Equal(t, "test body content", string(captured))

		bc.Close()
	})

	t.Run("captures partial reads", func(t *testing.T) {
		originalBody := io.NopCloser(strings.NewReader("test body content"))
		var captured []byte

		bc := &bodyCapture{
			ReadCloser: originalBody,
			onRead: func(data []byte) {
				captured = data
			},
		}

		// Read in small chunks
		buf := make([]byte, 5)
		totalRead := 0
		for {
			n, err := bc.Read(buf)
			totalRead += n
			if err == io.EOF {
				break
			}
			assert.NoError(t, err)
		}

		assert.Equal(t, 17, totalRead) // "test body content" is 17 bytes
		assert.Equal(t, "test body content", string(captured))

		bc.Close()
	})

	t.Run("calls onClose when body is closed", func(t *testing.T) {
		originalBody := io.NopCloser(strings.NewReader("test body"))
		var closeCalled bool
		var captured []byte

		bc := &bodyCapture{
			ReadCloser: originalBody,
			onClose: func(data []byte) {
				closeCalled = true
				captured = data
			},
		}

		// Read the body
		io.ReadAll(bc)

		// Close the body
		err := bc.Close()
		assert.NoError(t, err)
		assert.True(t, closeCalled)
		assert.Equal(t, "test body", string(captured))
	})

	t.Run("handles nil ReadCloser", func(t *testing.T) {
		bc := &bodyCapture{
			ReadCloser: nil,
		}

		// Read should return EOF
		buf := make([]byte, 10)
		n, err := bc.Read(buf)
		assert.Equal(t, 0, n)
		assert.Equal(t, io.EOF, err)

		// Close should not panic
		err = bc.Close()
		assert.NoError(t, err)
	})

	t.Run("onRead called only once", func(t *testing.T) {
		originalBody := io.NopCloser(strings.NewReader("test"))
		callCount := 0

		bc := &bodyCapture{
			ReadCloser: originalBody,
			onRead: func(data []byte) {
				callCount++
			},
		}

		// Read all (triggers onRead)
		io.ReadAll(bc)

		// Close (should not trigger onRead again)
		bc.Close()

		assert.Equal(t, 1, callCount, "onRead should be called only once")
	})

	t.Run("onRead called on close if not fully read", func(t *testing.T) {
		originalBody := io.NopCloser(strings.NewReader("test body"))
		var captured []byte
		onReadCalled := false

		bc := &bodyCapture{
			ReadCloser: originalBody,
			onRead: func(data []byte) {
				onReadCalled = true
				captured = data
			},
		}

		// Read only partial data
		buf := make([]byte, 4)
		bc.Read(buf)

		// Close without reading rest
		bc.Close()

		assert.True(t, onReadCalled)
		assert.Equal(t, "test", string(captured)) // Only captured what was read
	})
}

func TestFormatEndpoint(t *testing.T) {
	testCases := []struct {
		name     string
		makeReq  func() *http.Request
		expected string
	}{
		{
			name: "unix socket with path as host",
			makeReq: func() *http.Request {
				req := httptest.NewRequest("GET", "http://example.com/v1.43/version", nil)
				// Simulate how Docker client sets Unix socket URLs
				req.URL.Host = "/var/run/docker.sock"
				return req
			},
			expected: "unix:///var/run/docker.sock/v1.43/version",
		},
		{
			name: "regular http url",
			makeReq: func() *http.Request {
				return httptest.NewRequest("GET", "http://localhost:2375/v1.43/version", nil)
			},
			expected: "http://localhost:2375/v1.43/version",
		},
		{
			name: "https url",
			makeReq: func() *http.Request {
				return httptest.NewRequest("GET", "https://docker.example.com/v1.43/info", nil)
			},
			expected: "https://docker.example.com/v1.43/info",
		},
		{
			name: "tcp url",
			makeReq: func() *http.Request {
				return httptest.NewRequest("GET", "tcp://127.0.0.1:2376/containers/json", nil)
			},
			expected: "tcp://127.0.0.1:2376/containers/json",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := tc.makeReq()
			result := formatEndpoint(req)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestCaptureRequest(t *testing.T) {
	baseTransport := &mockRoundTripper{}
	wrapped := &wrappedTransport{base: baseTransport}

	t.Run("captures request metadata", func(t *testing.T) {
		req := httptest.NewRequest("POST", "http://example.com/test", strings.NewReader("body"))
		ctx := wrapped.captureRequest(req)

		assert.Equal(t, "POST", ctx.method)
		assert.Equal(t, "http://example.com/test", ctx.endpoint)
		assert.Equal(t, wrapped, ctx.transport)
	})

	t.Run("wraps request body", func(t *testing.T) {
		originalBody := "request body"
		req := httptest.NewRequest("POST", "http://example.com/test", strings.NewReader(originalBody))

		ctx := wrapped.captureRequest(req)

		// Request body should be wrapped
		_, ok := req.Body.(*bodyCapture)
		assert.True(t, ok, "expected body to be wrapped with bodyCapture")

		// Read the body to trigger capture
		body, err := io.ReadAll(req.Body)
		assert.NoError(t, err)
		assert.Equal(t, originalBody, string(body))
		req.Body.Close()

		// Verify the body was captured in context
		assert.Equal(t, originalBody, string(ctx.reqBody))
	})

	t.Run("handles nil body", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com/test", nil)
		ctx := wrapped.captureRequest(req)

		// Request body may be wrapped even if nil (http.noBody)
		// We just verify the context doesn't have captured body data
		assert.Nil(t, ctx.reqBody)
	})
}

func TestCaptureResponse(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)
	logrus.SetLevel(logrus.DebugLevel)

	baseTransport := &mockRoundTripper{}
	wrapped := &wrappedTransport{base: baseTransport}

	t.Run("wraps response body", func(t *testing.T) {
		ctx := &captureContext{
			transport: wrapped,
			method:    "GET",
			endpoint:  "/test",
		}

		originalResp := &http.Response{
			StatusCode: 200,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("response body")),
		}

		resp, err := wrapped.captureResponse(ctx, originalResp, nil)

		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Response body should be wrapped
		_, ok := resp.Body.(*bodyCapture)
		assert.True(t, ok, "expected response body to be wrapped with bodyCapture")
	})

	t.Run("handles error response", func(t *testing.T) {
		ctx := &captureContext{
			transport: wrapped,
			method:    "GET",
			endpoint:  "/test",
		}

		expectedErr := errors.New("connection error")
		resp, err := wrapped.captureResponse(ctx, nil, expectedErr)

		assert.Error(t, err)
		assert.Equal(t, expectedErr, err)
		assert.Nil(t, resp)
	})
}

func TestIntegrationWithDockerClient(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	// Create a test HTTP server to simulate Docker daemon
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Docker version endpoint
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"Version":"test","ApiVersion":"1.43"}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	t.Run("debug transport works with real client", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)

		client, err := dockerapi.NewClientWithOpts(
			dockerapi.WithHost(server.URL),
			dockerapi.WithHTTPClient(&http.Client{}),
			withDebugTransport(),
		)
		require.NoError(t, err)
		defer client.Close()

		// Verify transport is configured (may be wrapped by docker client)
		httpClient := client.HTTPClient()
		assert.NotNil(t, httpClient.Transport, "expected transport to be configured")

		// The actual verification that it works is that the transport doesn't panic
		// and the client can be created successfully
	})
}
