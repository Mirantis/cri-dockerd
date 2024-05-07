//go:build windows
// +build windows

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

package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"runtime"

	"golang.org/x/sys/windows/registry"

	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	dockerbackend "github.com/docker/docker/api/types/backend"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerfilters "github.com/docker/docker/api/types/filters"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// DefaultMemorySwap always returns 0 for no memory swap in a sandbox
func DefaultMemorySwap() int64 {
	return 0
}

func (ds *dockerService) updateCreateConfig(
	createConfig *dockerbackend.ContainerCreateConfig,
	config *runtimeapi.ContainerConfig,
	sandboxConfig *runtimeapi.PodSandboxConfig,
	podSandboxID string, securityOptSep rune, apiVersion *semver.Version) error {
	if networkMode := os.Getenv("CONTAINER_NETWORK"); networkMode != "" {
		createConfig.HostConfig.NetworkMode = dockercontainer.NetworkMode(networkMode)
	} else {
		// Todo: Refactor this call in future for calling methods directly in security_context.go
		modifyHostOptionsForContainer(nil, podSandboxID, createConfig.HostConfig)
	}

	// Apply Windows-specific options if applicable.
	if wc := config.GetWindows(); wc != nil {
		rOpts := wc.GetResources()
		if rOpts != nil {
			// Precedence and units for these are described at length in kuberuntime_container_windows.go - generateWindowsContainerConfig()
			createConfig.HostConfig.Resources = dockercontainer.Resources{
				Memory:    rOpts.MemoryLimitInBytes,
				CPUShares: rOpts.CpuShares,
				CPUCount:  rOpts.CpuCount,
				NanoCPUs:  rOpts.CpuMaximum * int64(runtime.NumCPU()) * (1e9 / 10000),
			}
		}

		// Apply security context.
		applyWindowsContainerSecurityContext(
			wc.GetSecurityContext(),
			createConfig.Config,
			createConfig.HostConfig,
		)
	}

	return nil
}

func (ds *dockerService) determinePodIPBySandboxID(sandboxID string) []string {
	opts := dockercontainer.ListOptions{
		All:     true,
		Filters: dockerfilters.NewArgs(),
	}

	f := NewDockerFilter(&opts.Filters)
	f.AddLabel(containerTypeLabelKey, containerTypeLabelContainer)
	f.AddLabel(sandboxIDLabelKey, sandboxID)
	containers, err := ds.client.ListContainers(opts)
	if err != nil {
		return nil
	}

	for _, c := range containers {
		r, err := ds.client.InspectContainer(c.ID)
		if err != nil {
			continue
		}

		// Versions and feature support
		// ============================
		// Windows version == Windows Server, Version 1709, Supports both sandbox and non-sandbox case
		// Windows version == Windows Server 2016   Support only non-sandbox case
		// Windows version < Windows Server 2016 is Not Supported

		// Sandbox support in Windows mandates CNI Plugin.
		// Presence of CONTAINER_NETWORK flag is considered as non-Sandbox cases here

		// Todo: Add a kernel version check for more validation

		if networkMode := os.Getenv("CONTAINER_NETWORK"); networkMode == "" {
			// On Windows, every container that is created in a Sandbox, needs to invoke CNI plugin again for adding the Network,
			// with the shared container name as NetNS info,
			// This is passed down to the platform to replicate some necessary information to the new container

			//
			// This place is chosen as a hack for now, since ds.getIP would end up calling CNI's addToNetwork
			// That is why addToNetwork is required to be idempotent

			// Instead of relying on this call, an explicit call to addToNetwork should be
			// done immediately after ContainerCreation, in case of Windows only. TBD Issue # to handle this

			// Do not return any IP, so that we would continue and get the IP of the Sandbox.
			// Windows 1709 and 1803 doesn't have the Namespace support, so getIP() is called
			// to replicate the DNS registry key to the Workload container (IP/Gateway/MAC is
			// set separately than DNS).
			ds.getIPs(sandboxID, r)
		} else {
			// ds.getIP will call the CNI plugin to fetch the IP
			if containerIPs := ds.getIPs(c.ID, r); len(containerIPs) != 0 {
				return containerIPs
			}
		}
	}

	return nil
}

func getNetworkNamespace(c *dockertypes.ContainerJSON) (string, error) {
	// Currently in windows there is no identifier exposed for network namespace
	// Like docker, the referenced container id is used to figure out the network namespace id internally by the platform
	// so returning the docker networkMode (which holds container:<ref containerid> for network namespace here
	return string(c.HostConfig.NetworkMode), nil
}

type containerCleanupInfo struct {
	gMSARegistryValueName string
}

// applyPlatformSpecificDockerConfig applies platform-specific configurations to a dockerbackend.ContainerCreateConfig struct.
// The containerCleanupInfo struct it returns will be passed as is to performPlatformSpecificContainerCleanup
// after either the container creation has failed or the container has been removed.
func (ds *dockerService) applyPlatformSpecificDockerConfig(
	request *runtimeapi.CreateContainerRequest,
	createConfig *dockerbackend.ContainerCreateConfig,
) (*containerCleanupInfo, error) {
	cleanupInfo := &containerCleanupInfo{}

	if err := applyGMSAConfig(request.GetConfig(), createConfig, cleanupInfo); err != nil {
		return nil, err
	}

	return cleanupInfo, nil
}

// applyGMSAConfig looks at the container's .Windows.SecurityContext.GMSACredentialSpec field; if present,
// it copies its contents to a unique registry value, and sets a SecurityOpt on the config pointing to that registry value.
// We use registry values instead of files since their location cannot change - as opposed to credential spec files,
// whose location could potentially change down the line, or even be unknown (eg if docker is not installed on the
// C: drive)
// When docker supports passing a credential spec's contents directly, we should switch to using that
// as it will avoid cluttering the registry - there is a moby PR out for this:
// https://github.com/moby/moby/pull/38777
func applyGMSAConfig(
	config *runtimeapi.ContainerConfig,
	createConfig *dockerbackend.ContainerCreateConfig,
	cleanupInfo *containerCleanupInfo,
) error {
	var credSpec string
	if config.Windows != nil && config.Windows.SecurityContext != nil {
		credSpec = config.Windows.SecurityContext.CredentialSpec
	}
	if credSpec == "" {
		return nil
	}

	valueName, err := copyGMSACredSpecToRegistryValue(credSpec)
	if err != nil {
		return err
	}

	if createConfig.HostConfig == nil {
		createConfig.HostConfig = &dockercontainer.HostConfig{}
	}

	createConfig.HostConfig.SecurityOpt = append(
		createConfig.HostConfig.SecurityOpt,
		"credentialspec=registry://"+valueName,
	)
	cleanupInfo.gMSARegistryValueName = valueName

	return nil
}

const (
	// same as https://github.com/moby/moby/blob/93d994e29c9cc8d81f1b0477e28d705fa7e2cd72/daemon/oci_windows.go#L23
	credentialSpecRegistryLocation = `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Virtualization\Containers\CredentialSpecs`
	// the prefix for the registry values we write GMSA cred specs to
	gMSARegistryValueNamePrefix = "k8s-cred-spec-"
	// the number of random bytes to generate suffixes for registry value names
	gMSARegistryValueNameSuffixRandomBytes = 40
)

// registryKey is an interface wrapper around `registry.Key`,
// listing only the methods we care about here.
// It's mainly useful to easily allow mocking the registry in tests.
type registryKey interface {
	SetStringValue(name, value string) error
	DeleteValue(name string) error
	ReadValueNames(n int) ([]string, error)
	Close() error
}

var registryCreateKeyFunc = func(baseKey registry.Key, path string, access uint32) (registryKey, bool, error) {
	return registry.CreateKey(baseKey, path, access)
}

// randomReader is only meant to ever be overridden for testing purposes,
// same idea as for `registryKey` above
var randomReader = rand.Reader

// gMSARegistryValueNamesRegex is the regex used to detect gMSA cred spec
// registry values in `removeAllGMSARegistryValues` below.
var gMSARegistryValueNamesRegex = regexp.MustCompile(
	fmt.Sprintf(
		"^%s[0-9a-f]{%d}$",
		gMSARegistryValueNamePrefix,
		2*gMSARegistryValueNameSuffixRandomBytes,
	),
)

// copyGMSACredSpecToRegistryKey copies the credential specs to a unique registry value, and returns its name.
func copyGMSACredSpecToRegistryValue(credSpec string) (string, error) {
	valueName, err := gMSARegistryValueName()
	if err != nil {
		return "", err
	}

	// write to the registry
	key, _, err := registryCreateKeyFunc(
		registry.LOCAL_MACHINE,
		credentialSpecRegistryLocation,
		registry.SET_VALUE,
	)
	if err != nil {
		return "", fmt.Errorf(
			"unable to open registry key %q: %v",
			credentialSpecRegistryLocation,
			err,
		)
	}
	defer key.Close()
	if err = key.SetStringValue(valueName, credSpec); err != nil {
		return "", fmt.Errorf(
			"unable to write into registry value %q/%q: %v",
			credentialSpecRegistryLocation,
			valueName,
			err,
		)
	}

	return valueName, nil
}

// gMSARegistryValueName computes the name of the registry value where to store the GMSA cred spec contents.
// The value's name is a purely random suffix appended to `gMSARegistryValueNamePrefix`.
func gMSARegistryValueName() (string, error) {
	randomSuffix, err := randomString(gMSARegistryValueNameSuffixRandomBytes)

	if err != nil {
		return "", fmt.Errorf("error when generating gMSA registry value name: %v", err)
	}

	return gMSARegistryValueNamePrefix + randomSuffix, nil
}

// randomString returns a random hex string.
func randomString(length int) (string, error) {
	randBytes := make([]byte, length)

	if n, err := randomReader.Read(randBytes); err != nil || n != length {
		if err == nil {
			err = fmt.Errorf("only got %v random bytes, expected %v", n, length)
		}
		return "", fmt.Errorf("unable to generate random string: %v", err)
	}

	return hex.EncodeToString(randBytes), nil
}

func removeGMSARegistryValue(cleanupInfo *containerCleanupInfo) error {
	if cleanupInfo == nil || cleanupInfo.gMSARegistryValueName == "" {
		return nil
	}

	key, _, err := registryCreateKeyFunc(
		registry.LOCAL_MACHINE,
		credentialSpecRegistryLocation,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("unable to open registry key %q: %v", credentialSpecRegistryLocation, err)
	}
	defer key.Close()
	if err = key.DeleteValue(cleanupInfo.gMSARegistryValueName); err != nil {
		return fmt.Errorf(
			"unable to remove registry value %q/%q: %v",
			credentialSpecRegistryLocation,
			cleanupInfo.gMSARegistryValueName,
			err,
		)
	}

	return nil
}

func removeAllGMSARegistryValues() (errors []error) {
	key, _, err := registryCreateKeyFunc(
		registry.LOCAL_MACHINE,
		credentialSpecRegistryLocation,
		registry.SET_VALUE,
	)
	if err != nil {
		return []error{
			fmt.Errorf("unable to open registry key %q: %v", credentialSpecRegistryLocation, err),
		}
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(0)
	if err != nil {
		return []error{
			fmt.Errorf(
				"unable to list values under registry key %q: %v",
				credentialSpecRegistryLocation,
				err,
			),
		}
	}

	for _, valueName := range valueNames {
		if gMSARegistryValueNamesRegex.MatchString(valueName) {
			if err = key.DeleteValue(valueName); err != nil {
				errors = append(
					errors,
					fmt.Errorf(
						"unable to remove registry value %q/%q: %v",
						credentialSpecRegistryLocation,
						valueName,
						err,
					),
				)
			}
		}
	}

	return
}

func (ds *dockerService) performPlatformSpecificContainerForContainer(
	containerID string,
) (errors []error) {
	if cleanupInfo, present := ds.getContainerCleanupInfo(containerID); present {
		errors = ds.performPlatformSpecificContainerCleanupAndLogErrors(containerID, cleanupInfo)

		if len(errors) == 0 {
			ds.clearContainerCleanupInfo(containerID)
		}
	}

	return
}

func (ds *dockerService) performPlatformSpecificContainerCleanupAndLogErrors(
	containerNameOrID string,
	cleanupInfo *containerCleanupInfo,
) []error {
	if cleanupInfo == nil {
		return nil
	}

	errors := ds.performPlatformSpecificContainerCleanup(cleanupInfo)
	for _, err := range errors {
		logrus.Infof("Error when cleaning up after container %s: %v", containerNameOrID, err)
	}

	return errors
}
