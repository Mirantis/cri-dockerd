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
	"context"
	"fmt"
	"path/filepath"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// CreateContainer creates a new container in the given PodSandbox
// Docker cannot store the log to an arbitrary location (yet), so we create an
// symlink at LogPath, linking to the actual path of the log.
func (ds *dockerService) CreateContainer(
	_ context.Context,
	r *v1.CreateContainerRequest,
) (*v1.CreateContainerResponse, error) {
	podSandboxID := r.PodSandboxId
	config := r.GetConfig()
	sandboxConfig := r.GetSandboxConfig()

	if config == nil {
		return nil, fmt.Errorf("container config is nil")
	}
	if sandboxConfig == nil {
		return nil, fmt.Errorf("sandbox config is nil for container %q", config.Metadata.Name)
	}

	labels := makeLabels(config.GetLabels(), config.GetAnnotations())
	// Apply a the container type label.
	labels[containerTypeLabelKey] = containerTypeLabelContainer
	// Write the container log path in the labels.
	labels[containerLogPathLabelKey] = filepath.Join(sandboxConfig.LogDirectory, config.LogPath)
	// Write the sandbox ID in the labels.
	labels[sandboxIDLabelKey] = podSandboxID

	apiVersion, err := ds.getDockerAPIVersion()
	if err != nil {
		return nil, fmt.Errorf("unable to get the docker API version: %v", err)
	}

	image := ""
	if iSpec := config.GetImage(); iSpec != nil {
		image = iSpec.Image
	}
	containerName := makeContainerName(sandboxConfig, config)
	terminationMessagePath, _ := config.Annotations["io.kubernetes.container.terminationMessagePath"]
	createConfig := types.ContainerCreateConfig{
		Name: containerName,
		Config: &container.Config{
			Entrypoint: strslice.StrSlice(config.Command),
			Cmd:        strslice.StrSlice(config.Args),
			Env:        libdocker.GenerateEnvList(config.GetEnvs()),
			Image:      image,
			WorkingDir: config.WorkingDir,
			Labels:     labels,
			// Interactive containers:
			OpenStdin: config.Stdin,
			StdinOnce: config.StdinOnce,
			Tty:       config.Tty,
			// Disable Docker's health check until we officially support it
			// (https://github.com/kubernetes/kubernetes/issues/25829).
			Healthcheck: &container.HealthConfig{
				Test: []string{"NONE"},
			},
		},
		HostConfig: &container.HostConfig{
			Binds: libdocker.GenerateMountBindings(config.GetMounts(), terminationMessagePath),
			RestartPolicy: container.RestartPolicy{
				Name: "no",
			},
		},
	}

	hc := createConfig.HostConfig
	err = ds.updateCreateConfig(
		&createConfig,
		config,
		sandboxConfig,
		podSandboxID,
		securityOptSeparator,
		apiVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update container create config: %v", err)
	}
	// Set devices for container.
	devices := make([]container.DeviceMapping, len(config.Devices))
	for i, device := range config.Devices {
		devices[i] = container.DeviceMapping{
			PathOnHost:        device.HostPath,
			PathInContainer:   device.ContainerPath,
			CgroupPermissions: device.Permissions,
		}
	}
	hc.Resources.Devices = devices

	securityOpts, err := ds.getSecurityOpts(
		config.GetLinux().GetSecurityContext().GetSeccompProfilePath(),
		securityOptSeparator,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to generate security options for container %q: %v",
			config.Metadata.Name,
			err,
		)
	}

	hc.SecurityOpt = append(hc.SecurityOpt, securityOpts...)

	cleanupInfo, err := ds.applyPlatformSpecificDockerConfig(r, &createConfig)
	if err != nil {
		return nil, err
	}

	createResp, createErr := ds.client.CreateContainer(createConfig)
	if createErr != nil {
		createResp, createErr = recoverFromCreationConflictIfNeeded(
			ds.client,
			createConfig,
			createErr,
		)
	}

	if createResp != nil {
		containerID := createResp.ID

		if cleanupInfo != nil {
			// we don't perform the clean up just yet at that could destroy information
			// needed for the container to start (e.g. Windows credentials stored in
			// registry keys); instead, we'll clean up when the container gets removed
			ds.setContainerCleanupInfo(containerID, cleanupInfo)
		}
		return &v1.CreateContainerResponse{ContainerId: containerID}, nil
	}

	// the creation failed, let's clean up right away - we ignore any errors though,
	// this is best effort
	ds.performPlatformSpecificContainerCleanupAndLogErrors(containerName, cleanupInfo)

	return nil, createErr
}
