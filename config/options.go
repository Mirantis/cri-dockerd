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
	"github.com/spf13/pflag"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ContainerRuntimeOptions contains runtime options
type ContainerRuntimeOptions struct {
	// General options.

	//// driver that the kubelet uses to manipulate cgroups on the host (cgroupfs or systemd)
	CgroupDriver string
	// RuntimeCgroups that container runtime is expected to be isolated in.
	RuntimeCgroups string

	// Docker-specific options.

	// CriDockerdRootDirectory is the path to the cri-dockerd root directory. Defaults to
	// /var/lib/cri-dockerd if unset. Exposed for integration testing (e.g. in OpenShift).
	CriDockerdRootDirectory string
	// PodSandboxImage is the image whose network/ipc namespaces
	// containers in each pod will use.
	PodSandboxImage string
	// DockerEndpoint is the path to the docker endpoint to communicate with.
	DockerEndpoint string
	// If no pulling progress is made before the deadline imagePullProgressDeadline,
	// the image pulling will be cancelled. Defaults to 1m0s.
	// +optional
	ImagePullProgressDeadline v1.Duration
	// runtimeRequestTimeout is the timeout for all runtime requests except long-running
	// requests - pull, logs, exec and attach.
	RuntimeRequestTimeout v1.Duration
	// streamingConnectionIdleTimeout is the maximum time a streaming connection
	// can be idle before the connection is automatically closed.
	StreamingConnectionIdleTimeout v1.Duration

	// StreamingBindAddr is the address to bind the CRI streaming server to.
	// If not specified, it will bind to all addresses
	StreamingBindAddr string

	// Network plugin options.

	// The CIDR to use for pod IP addresses, only used in standalone mode.
	// In cluster mode, this is obtained from the master.
	PodCIDR string
	// enableIPv6DualStack allows dual-homed pods
	IPv6DualStackEnabled bool
	// networkPluginName is the name of the network plugin to be invoked for
	// various events in kubelet/pod lifecycle
	NetworkPluginName string
	// NetworkPluginMTU is the MTU to be passed to the network plugin,
	// and overrides the default MTU for cases where it cannot be automatically
	// computed (such as IPSEC).
	NetworkPluginMTU int32
	// CNIConfDir is the full path of the directory in which to search for
	// CNI config files
	CNIConfDir string
	// CNIBinDir is the full path of the directory in which to search for
	// CNI plugin binaries
	CNIBinDir string
	// CNICacheDir is the full path of the directory in which CNI should store
	// cache files
	CNICacheDir string
	// HairpinMode is the mode used to allow endpoints of a Service to load
	// balance back to themselves if they should try to access their own Service
	HairpinMode HairpinMode
}

// AddFlags has the set of flags needed by cri-dockerd
func (s *ContainerRuntimeOptions) AddFlags(fs *pflag.FlagSet) {
	// General settings.
	fs.StringVar(
		&s.RuntimeCgroups,
		"runtime-cgroups",
		s.RuntimeCgroups,
		"Optional absolute name of cgroups to create and run the runtime in.",
	)

	// Docker-specific settings.
	fs.StringVar(
		&s.CriDockerdRootDirectory,
		"cri-dockerd-root-directory",
		s.CriDockerdRootDirectory,
		"Path to the cri-dockerd root directory.",
	)
	fs.StringVar(
		&s.PodSandboxImage,
		"pod-infra-container-image",
		s.PodSandboxImage,
		"The image whose network/ipc namespaces containers in each pod will use",
	)
	fs.StringVar(
		&s.DockerEndpoint,
		"docker-endpoint",
		s.DockerEndpoint,
		"Use this for the docker endpoint to communicate with.",
	)
	fs.DurationVar(
		&s.ImagePullProgressDeadline.Duration,
		"image-pull-progress-deadline",
		s.ImagePullProgressDeadline.Duration,
		"If no pulling progress is made before this deadline, the image pulling will be cancelled.",
	)
	fs.DurationVar(
		&s.RuntimeRequestTimeout.Duration,
		"runtime-request-timeout",
		s.RuntimeRequestTimeout.Duration,
		"If no runtime progress is made before this deadline, the operation will be cancelled.",
	)

	fs.StringVar(
		&s.StreamingBindAddr,
		"streaming-bind-addr",
		s.StreamingBindAddr,
		"The address to bind the CRI streaming server to. If not specified, it will bind to all addresses.",
	)
	// Network plugin settings for Docker.
	fs.StringVar(
		&s.PodCIDR,
		"pod-cidr",
		s.PodCIDR,
		"The CIDR to use for pod IP addresses, only used in standalone mode.  In cluster mode, this is obtained from the master. For IPv6, the maximum number of IP's allocated is 65536",
	)
	fs.BoolVar(
		&s.IPv6DualStackEnabled,
		"ipv6-dual-stack",
		s.IPv6DualStackEnabled,
		"Enable IPv6 dual stack support",
	)
	fs.StringVar(
		&s.NetworkPluginName,
		"network-plugin",
		s.NetworkPluginName,
		"The name of the network plugin to be invoked for various events in kubelet/pod lifecycle.",
	)
	fs.StringVar(
		&s.CNIConfDir,
		"cni-conf-dir",
		s.CNIConfDir,
		"The full path of the directory in which to search for CNI config files",
	)
	fs.StringVar(
		&s.CNIBinDir,
		"cni-bin-dir",
		s.CNIBinDir,
			"A comma-separated list of full paths of directories in which to search for CNI plugin binaries.",
	)
	fs.StringVar(
		&s.CNICacheDir,
		"cni-cache-dir",
		s.CNICacheDir,
		"The full path of the directory in which CNI should store cache files.",
	)
	fs.Int32Var(
		&s.NetworkPluginMTU,
		"network-plugin-mtu",
		s.NetworkPluginMTU,
		"<Warning: Alpha feature> The MTU to be passed to the network plugin, to override the default. Set to 0 to use the default 1460 MTU.",
	)
	fs.Var(
		&HairpinModeVar,
		"hairpin-mode",
		"<Warning: Alpha feature> The mode of hairpin to use.",
	)
}
