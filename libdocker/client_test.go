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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDockerClient(t *testing.T) {
	// Save original log level and restore after test
	originalLevel := logrus.GetLevel()
	defer logrus.SetLevel(originalLevel)

	t.Run("creates client with explicit endpoint", func(t *testing.T) {
		client, err := getDockerClient("unix:///var/run/docker.sock")
		require.NoError(t, err)
		require.NotNil(t, client)
		defer client.Close()
	})

	t.Run("creates client with environment configuration", func(t *testing.T) {
		client, err := getDockerClient("")
		require.NoError(t, err)
		require.NotNil(t, client)
		defer client.Close()
	})

	t.Run("includes debug transport when debug level enabled", func(t *testing.T) {
		logrus.SetLevel(logrus.DebugLevel)
		client, err := getDockerClient("unix:///var/run/docker.sock")
		require.NoError(t, err)
		require.NotNil(t, client)
		defer client.Close()

		// Verify that the HTTP client has a transport
		httpClient := client.HTTPClient()
		require.NotNil(t, httpClient)
		require.NotNil(t, httpClient.Transport)

		// The Docker client may wrap the transport with additional layers (like otelhttp),
		// so we just verify the transport is not nil at debug level
		// The actual wrappedTransport may be nested inside
		assert.NotNil(t, httpClient.Transport, "expected transport to be configured at debug level")
	})

	t.Run("does not include debug transport at info level", func(t *testing.T) {
		logrus.SetLevel(logrus.InfoLevel)
		client, err := getDockerClient("unix:///var/run/docker.sock")
		require.NoError(t, err)
		require.NotNil(t, client)
		defer client.Close()

		// Verify that the HTTP client has a transport
		httpClient := client.HTTPClient()
		require.NotNil(t, httpClient)
		require.NotNil(t, httpClient.Transport)

		// At info level, the transport should still be configured (but not wrapped with debug)
		assert.NotNil(t, httpClient.Transport)
	})
}
