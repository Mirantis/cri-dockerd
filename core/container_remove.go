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
	"time"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// RemoveContainer removes the container.
func (ds *dockerService) RemoveContainer(
	_ context.Context,
	r *v1.RemoveContainerRequest,
) (*v1.RemoveContainerResponse, error) {
	// Ideally, log lifecycle should be independent of container lifecycle.
	// However, docker will remove container log after container is removed,
	// we can't prevent that now, so we also clean up the symlink here.
	err := ds.removeContainerLogSymlink(r.ContainerId)
	if err != nil {
		return nil, err
	}
	errors := ds.performPlatformSpecificContainerForContainer(r.ContainerId)
	if len(errors) != 0 {
		return nil, fmt.Errorf(
			"failed to run platform-specific clean ups for container %q: %v",
			r.ContainerId,
			errors,
		)
	}
	err = ds.client.RemoveContainer(
		r.ContainerId,
		dockercontainer.RemoveOptions{RemoveVolumes: true, Force: true},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to remove container %q: %v", r.ContainerId, err)
	}

	return &v1.RemoveContainerResponse{}, nil
}

func (ds *dockerService) getContainerCleanupInfo(containerID string) (*containerCleanupInfo, bool) {
	ds.cleanupInfosLock.RLock()
	defer ds.cleanupInfosLock.RUnlock()
	info, ok := ds.containerCleanupInfos[containerID]
	return info, ok
}

func (ds *dockerService) setContainerCleanupInfo(containerID string, info *containerCleanupInfo) {
	ds.cleanupInfosLock.Lock()
	defer ds.cleanupInfosLock.Unlock()
	ds.containerCleanupInfos[containerID] = info
}

func (ds *dockerService) clearContainerCleanupInfo(containerID string) {
	ds.cleanupInfosLock.Lock()
	defer ds.cleanupInfosLock.Unlock()
	delete(ds.containerCleanupInfos, containerID)
}

func getContainerTimestamps(r *types.ContainerJSON) (time.Time, time.Time, time.Time, error) {
	var createdAt, startedAt, finishedAt time.Time
	var err error

	createdAt, err = libdocker.ParseDockerTimestamp(r.Created)
	if err != nil {
		return createdAt, startedAt, finishedAt, err
	}
	startedAt, err = libdocker.ParseDockerTimestamp(r.State.StartedAt)
	if err != nil {
		return createdAt, startedAt, finishedAt, err
	}
	finishedAt, err = libdocker.ParseDockerTimestamp(r.State.FinishedAt)
	if err != nil {
		return createdAt, startedAt, finishedAt, err
	}
	return createdAt, startedAt, finishedAt, nil
}
