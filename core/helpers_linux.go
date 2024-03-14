//go:build linux
// +build linux

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
	dockercontainer "github.com/docker/docker/api/types/container"
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
	// Apply Linux-specific options if applicable.
	if lc := config.GetLinux(); lc != nil {
		rOpts := lc.GetResources()
		if rOpts != nil {
			createConfig.HostConfig.Resources = dockercontainer.Resources{
				// Memory and MemorySwap are set to the same value, this prevents containers from using any swap.
				Memory:     rOpts.MemoryLimitInBytes,
				MemorySwap: rOpts.MemoryLimitInBytes,
				CPUShares:  rOpts.CpuShares,
				CPUQuota:   rOpts.CpuQuota,
				CPUPeriod:  rOpts.CpuPeriod,
				CpusetCpus: rOpts.CpusetCpus,
				CpusetMems: rOpts.CpusetMems,
			}
			createConfig.HostConfig.OomScoreAdj = int(rOpts.OomScoreAdj)
		}
		// Note: ShmSize is handled in kube_docker_client.go

		// Apply security context.
		if err := applyContainerSecurityContext(lc, podSandboxID, createConfig.Config, createConfig.HostConfig, securityOptSep); err != nil {
			return fmt.Errorf(
				"failed to apply container security context for container %q: %v",
				config.Metadata.Name,
				err,
			)
		}
	}

	// Apply cgroupsParent derived from the sandbox config.
	if lc := sandboxConfig.GetLinux(); lc != nil {
		// Apply Cgroup options.
		cgroupParent, err := ds.GenerateExpectedCgroupParent(lc.CgroupParent)
		if err != nil {
			return fmt.Errorf(
				"failed to generate cgroup parent in expected syntax for container %q: %v",
				config.Metadata.Name,
				err,
			)
		}
		createConfig.HostConfig.CgroupParent = cgroupParent
	}

	return nil
}

func (ds *dockerService) determinePodIPBySandboxID(uid string) []string {
	return nil
}

func getNetworkNamespace(c *dockertypes.ContainerJSON) (string, error) {
	if c.State.Pid == 0 {
		// Docker reports pid 0 for an exited container.
		return "", fmt.Errorf("cannot find network namespace for the terminated container %q", c.ID)
	}
	return fmt.Sprintf(dockerNetNSFmt, c.State.Pid), nil
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
