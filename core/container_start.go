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
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// StartContainer starts the container.
func (ds *dockerService) StartContainer(
	_ context.Context,
	r *v1.StartContainerRequest,
) (*v1.StartContainerResponse, error) {
	err := ds.client.StartContainer(r.ContainerId)

	// Create container log symlink for all containers (including failed ones).
	if linkError := ds.createContainerLogSymlink(r.ContainerId); linkError != nil {
		// Do not stop the container if we failed to create symlink because:
		//   1. This is not a critical failure.
		//   2. We don't have enough information to properly stop container here.
		// Kubelet will surface this error to user via an event.
		return nil, linkError
	}

	if err != nil {
		err = transformStartContainerError(err)
		return nil, fmt.Errorf("failed to start container %q: %v", r.ContainerId, err)
	}

	return &v1.StartContainerResponse{}, nil
}

// transformStartContainerError does regex parsing on returned error
// for where container runtimes are giving less than ideal error messages.
func transformStartContainerError(err error) error {
	if err == nil {
		return nil
	}
	matches := startRE.FindStringSubmatch(err.Error())
	if len(matches) > 0 {
		return fmt.Errorf("executable not found in $PATH")
	}
	return err
}
