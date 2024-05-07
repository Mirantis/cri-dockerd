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

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/utils/errors"
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// RemovePodSandbox removes the sandbox. If there are running containers in the
// sandbox, they should be forcibly removed.
func (ds *dockerService) RemovePodSandbox(
	ctx context.Context,
	r *v1.RemovePodSandboxRequest,
) (*v1.RemovePodSandboxResponse, error) {
	podSandboxID := r.PodSandboxId
	var errs []error

	opts := dockercontainer.ListOptions{All: true}

	opts.Filters = filters.NewArgs()
	f := NewDockerFilter(&opts.Filters)
	f.AddLabel(sandboxIDLabelKey, podSandboxID)

	containers, err := ds.client.ListContainers(opts)
	if err != nil {
		errs = append(errs, err)
	}

	// Remove all containers in the sandbox.
	for i := range containers {
		if _, err := ds.RemoveContainer(ctx, &v1.RemoveContainerRequest{ContainerId: containers[i].ID}); err != nil &&
			!libdocker.IsContainerNotFoundError(err) {
			errs = append(errs, err)
		}
	}

	// Remove the sandbox container.
	err = ds.client.RemoveContainer(
		podSandboxID,
		dockercontainer.RemoveOptions{RemoveVolumes: true, Force: true},
	)
	if err == nil || libdocker.IsContainerNotFoundError(err) {
		// Only clear network ready when the sandbox has actually been
		// removed from docker or doesn't exist
		ds.clearNetworkReady(podSandboxID)
	} else {
		errs = append(errs, err)
	}

	// Remove the checkpoint of the sandbox.
	if err := ds.checkpointManager.RemoveCheckpoint(podSandboxID); err != nil {
		errs = append(errs, err)
	}
	if len(errs) == 0 {
		return &v1.RemovePodSandboxResponse{}, nil
	}
	return nil, errors.NewAggregate(errs)
}
