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
		httpClient.Transport = newDebugTransport(httpClient.Transport)
		// Replace the client's HTTP client with our wrapped version
		// Note: This must be done by replacing c.client directly since HTTPClient() returns a copy
		return dockerapi.WithHTTPClient(httpClient)(c)
	}
}

func newDebugTransport(baseTransport http.RoundTripper) http.RoundTripper {
	return &debugTransport{base: baseTransport}
}

type debugTransport struct {
	base http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	endpoint := formatEndpoint(req)
	req.Body = &interceptor{
		ReadCloser: req.Body,
		body:       []byte{},
		complete: func(body []byte) {
			logrus.Debugf("(dockerapi) %s %s: %s", req.Method, endpoint, string(body))
		},
	}
	resp, err := t.base.RoundTrip(req)
	logrus.Debugf("(dockerapi) %s %s: %s", req.Method, endpoint, func() string {
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return resp.Status
	}())
	return resp, err
}

func formatEndpoint(req *http.Request) string {
	// Check if this is a Unix socket (host is a file path starting with /)
	if host := req.URL.Host; host != "" && host[0] == '/' {
		return fmt.Sprintf("unix://%s%s", host, req.URL.Path)
	}
	return req.URL.String()
}

type interceptor struct {
	io.ReadCloser
	body     []byte
	complete func(body []byte)
}

func (p *interceptor) Read(b []byte) (int, error) {
	if p.ReadCloser == nil {
		return 0, io.EOF
	}
	n, err := p.ReadCloser.Read(b)
	if n > 0 {
		p.body = append(p.body, b[:n]...)
	}
	return n, err
}

func (p *interceptor) Close() error {
	if p.complete != nil {
		p.complete(p.body)
	}
	if p.ReadCloser == nil {
		return nil
	}
	return p.ReadCloser.Close()
}
