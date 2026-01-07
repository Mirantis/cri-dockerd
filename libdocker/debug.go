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
	"fmt"
	"io"
	"net/http"

	dockerapi "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

// withDebugTransport returns an Opt that wraps the existing HTTP transport with debug logging
func withDebugTransport() dockerapi.Opt {
	return func(c *dockerapi.Client) error {
		// Get a copy of the HTTP client and wrap its transport
		// This preserves socket configuration (Unix, TCP, HTTPS, etc.) from previous options
		httpClient := c.HTTPClient()
		httpClient.Transport = wrapTransport(httpClient.Transport)
		// Replace the client's HTTP client with our wrapped version
		// Note: This must be done by replacing c.client directly since HTTPClient() returns a copy
		return dockerapi.WithHTTPClient(httpClient)(c)
	}
}

func wrapTransport(baseTransport http.RoundTripper) http.RoundTripper {
	return &wrappedTransport{base: baseTransport}
}

type wrappedTransport struct {
	base http.RoundTripper
}

func (t *wrappedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Capture request and prepare for logging
	ctx := t.captureRequest(req)

	// Execute request
	resp, err := t.base.RoundTrip(req)

	// Capture response and log
	return t.captureResponse(ctx, resp, err)
}

func (t *wrappedTransport) captureRequest(req *http.Request) *captureContext {
	ctx := &captureContext{
		transport: t,
		method:    req.Method,
		endpoint:  formatEndpoint(req),
	}

	// Wrap request body to capture it
	if req.Body != nil {
		req.Body = &bodyCapture{
			ReadCloser: req.Body,
			onRead: func(captured []byte) {
				ctx.reqBody = captured
			},
		}
	}

	return ctx
}

func (t *wrappedTransport) captureResponse(ctx *captureContext, resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		ctx.log("", nil, err)
		return resp, err
	}

	// Wrap response body to capture and log when closed
	resp.Body = &bodyCapture{
		ReadCloser: resp.Body,
		onClose: func(capturedBody []byte) {
			ctx.log(resp.Status, capturedBody, nil)
		},
	}

	return resp, nil
}

// captureContext holds captured request data for logging
type captureContext struct {
	transport *wrappedTransport
	method    string
	endpoint  string
	reqBody   []byte
}

func (ctx *captureContext) log(respStatus string, respBody []byte, err error) {
	// Check if we should log with bodies (trace level) or without (debug level)
	if logrus.GetLevel() >= logrus.TraceLevel {
		ctx.logTrace(respStatus, respBody, err)
	} else {
		ctx.logDebug(respStatus, err)
	}
}

func (ctx *captureContext) logDebug(respStatus string, err error) {
	// Debug level: method + endpoint + status only
	if err != nil {
		logrus.Debugf("(dockerapi) %s %s => error: %v", ctx.method, ctx.endpoint, err)
	} else {
		logrus.Debugf("(dockerapi) %s %s => %s", ctx.method, ctx.endpoint, respStatus)
	}
}

func (ctx *captureContext) logTrace(respStatus string, respBody []byte, err error) {
	// Trace level: full request/response bodies
	reqBodyStr := string(ctx.reqBody)
	if reqBodyStr == "" {
		reqBodyStr = "(empty)"
	}

	if err != nil {
		logrus.Tracef("(dockerapi) %s %s: %s => error: %v", ctx.method, ctx.endpoint, reqBodyStr, err)
	} else {
		respBodyStr := string(respBody)
		if respBodyStr == "" {
			respBodyStr = "(empty)"
		}
		logrus.Tracef("(dockerapi) %s %s: %s => %s %s", ctx.method, ctx.endpoint, reqBodyStr, respStatus, respBodyStr)
	}
}

// bodyCapture captures request or response body for logging
type bodyCapture struct {
	io.ReadCloser
	captured []byte
	onRead   func([]byte) // Called when body is fully read
	onClose  func([]byte) // Called when body is closed
}

func (b *bodyCapture) Read(p []byte) (int, error) {
	if b.ReadCloser == nil {
		return 0, io.EOF
	}
	n, err := b.ReadCloser.Read(p)
	if n > 0 {
		b.captured = append(b.captured, p[:n]...)
	}
	// If we hit EOF and have an onRead callback, call it once
	if err == io.EOF && b.onRead != nil {
		b.onRead(b.captured)
		b.onRead = nil // Prevent double-call in Close()
	}
	return n, err
}

func (b *bodyCapture) Close() error {
	// Call onRead if we haven't yet (body closed without being fully read)
	if b.onRead != nil {
		b.onRead(b.captured)
		b.onRead = nil // Prevent double-call
	}

	var err error
	if b.ReadCloser != nil {
		err = b.ReadCloser.Close()
	}

	if b.onClose != nil {
		b.onClose(b.captured)
	}
	return err
}

func formatEndpoint(req *http.Request) string {
	// Check if this is a Unix socket (host is a file path starting with /)
	if host := req.URL.Host; host != "" && host[0] == '/' {
		return fmt.Sprintf("unix://%s%s", host, req.URL.Path)
	}
	return req.URL.String()
}
