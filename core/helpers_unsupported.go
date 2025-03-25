//go:build !linux && !windows
// +build !linux,!windows

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
	"fmt"

	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	dockerbackend "github.com/docker/docker/api/types/backend"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// DefaultMemorySwap always returns -1 for no memory swap in a sandbox
func DefaultMemorySwap() int64 {
	return -1
}

func (ds *dockerService) getSecurityOpts(
	seccompProfile *runtimeapi.SecurityProfile, privileged bool,
	separator rune,
) ([]string, error) {
	logrus.Info("getSecurityOpts is unsupported in this build")
	return nil, nil
}

func (ds *dockerService) getSandBoxSecurityOpts(separator rune) []string {
	logrus.Info("getSandBoxSecurityOpts is unsupported in this build")
	return nil
}

func (ds *dockerService) updateCreateConfig(
	createConfig *dockerbackend.ContainerCreateConfig,
	config *runtimeapi.ContainerConfig,
	sandboxConfig *runtimeapi.PodSandboxConfig,
	podSandboxID string, securityOptSep rune, apiVersion *semver.Version) error {
	logrus.Info("updateCreateConfig is unsupported in this build")
	return nil
}

func (ds *dockerService) determinePodIPBySandboxID(uid string) []string {
	logrus.Info("determinePodIPBySandboxID is unsupported in this build")
	return nil
}

func getNetworkNamespace(c *dockertypes.ContainerJSON) (string, error) {
	return "", fmt.Errorf("unsupported platform")
}

type containerCleanupInfo struct{}

// applyPlatformSpecificDockerConfig applies platform-specific configurations to a dockerbackend.ContainerCreateConfig struct.
// The containerCleanupInfo struct it returns will be passed as is to performPlatformSpecificContainerCleanup
// after either the container creation has failed or the container has been removed.
func (ds *dockerService) applyPlatformSpecificDockerConfig(
	*runtimeapi.CreateContainerRequest,
	*dockerbackend.ContainerCreateConfig,
) (*containerCleanupInfo, error) {
	return nil, nil
}

// performPlatformSpecificContainerCleanup is responsible for doing any platform-specific cleanup
// after either the container creation has failed or the container has been removed.
func (ds *dockerService) performPlatformSpecificContainerCleanup(
	cleanupInfo *containerCleanupInfo,
) (errors []error) {
	return
}

// platformSpecificContainerInitCleanup is called when cri-dockerd
// is starting, and is meant to clean up any cruft left by previous runs
// creating containers.
// Errors are simply logged, but don't prevent cri-dockerd from starting.
func (ds *dockerService) platformSpecificContainerInitCleanup() (errors []error) {
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
