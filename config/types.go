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

package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

// NetworkPluginSettings is the subset of kubelet runtime args we pass
// to the container runtime so it can probe for network plugins.
// In the future we will feed these directly to a standalone container
// runtime process.w
type NetworkPluginSettings struct {
	// HairpinMode is best described by comments surrounding the kubelet arg
	HairpinMode HairpinMode
	// NonMasqueradeCIDR is the range of ips which should *not* be included
	// in any MASQUERADE rules applied by the plugin
	NonMasqueradeCIDR string
	// PluginName is the name of the plugin, runtime shim probes for
	PluginName string
	// PluginBinDirString is a list of directories delimited by commas, in
	// which the binaries for the plugin with PluginName may be found.
	PluginBinDirString string
	// PluginBinDirs is an array of directories in which the binaries for
	// the plugin with PluginName may be found. The admin is responsible for
	// provisioning these binaries before-hand.
	PluginBinDirs []string
	// PluginConfDir is the directory in which the admin places a CNI conf.
	// Depending on the plugin, this may be an optional field, eg: kubenet
	// generates its own plugin conf.
	PluginConfDir string
	// PluginCacheDir is the directory in which CNI should store cache files.
	PluginCacheDir string
	// MTU is the desired MTU for network devices created by the plugin.
	MTU int
}

// enableIPv6DualStack allows dual-homed pods
var IPv6DualStackEnabled bool

// ClientConfig is parameters used to initialize docker client
type ClientConfig struct {
	DockerEndpoint            string
	RuntimeRequestTimeout     time.Duration
	ImagePullProgressDeadline time.Duration

	// Configuration for fake docker client
	EnableSleep       bool
	WithTraceDisabled bool
}

// NewDockerClientFromConfig create a docker client from given configure
// return nil if nil configure is given.
func NewDockerClientFromConfig(config *ClientConfig) libdocker.DockerClientInterface {
	if config != nil {
		// Create docker client.
		client := libdocker.ConnectToDockerOrDie(
			config.DockerEndpoint,
			config.RuntimeRequestTimeout,
			config.ImagePullProgressDeadline,
		)
		return client
	}

	return nil
}

// PortMapping is the port mapping configurations of a sandbox.
type PortMapping struct {
	// Protocol of the port mapping.
	Protocol *Protocol `json:"protocol,omitempty"`
	// Port number within the container.
	ContainerPort *int32 `json:"container_port,omitempty"`
	// Port number on the host.
	HostPort *int32 `json:"host_port,omitempty"`
	// Host ip to expose.
	HostIP string `json:"host_ip,omitempty"`
}

// ContainerID is a type that identifies a container.
type ContainerID struct {
	// The type of the container runtime.
	Type string
	// The identification of the container.
	ID string
}

// ParseString converts given string into ContainerID
func (c *ContainerID) ParseString(data string) error {
	// Trim the quotes and split the type and ID.
	parts := strings.Split(strings.Trim(data, "\""), "://")
	if len(parts) != 2 {
		return fmt.Errorf("invalid container ID: %q", data)
	}
	c.Type, c.ID = parts[0], parts[1]
	return nil
}

// BuildContainerID returns the ContainerID given type and id.
func BuildContainerID(typ, ID string) ContainerID {
	return ContainerID{Type: typ, ID: ID}
}
