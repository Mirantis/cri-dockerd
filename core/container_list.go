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
	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ListContainers lists all containers matching the filter.
func (ds *dockerService) ListContainers(
	_ context.Context,
	r *v1.ListContainersRequest,
) (*v1.ListContainersResponse, error) {
	filter := r.GetFilter()
	opts := dockercontainer.ListOptions{All: true}

	opts.Filters = filters.NewArgs()
	f := NewDockerFilter(&opts.Filters)
	// Add filter to get *only* (non-sandbox) containers.
	f.AddLabel(containerTypeLabelKey, containerTypeLabelContainer)

	if filter != nil {
		if filter.Id != "" {
			f.Add("id", filter.Id)
		}
		if filter.State != nil {
			f.Add("status", toDockerContainerStatus(filter.GetState().State))
		}
		if filter.PodSandboxId != "" {
			f.AddLabel(sandboxIDLabelKey, filter.PodSandboxId)
		}

		if filter.LabelSelector != nil {
			for k, v := range filter.LabelSelector {
				f.AddLabel(k, v)
			}
		}
	}
	containers, err := ds.client.ListContainers(opts)
	if err != nil && !libdocker.IsContainerNotFoundError(err) {
		return nil, err
	}
	// Convert docker to runtime api containers.
	result := []*v1.Container{}
	for i := range containers {
		c := containers[i]

		converted, err := toRuntimeAPIContainer(&c)
		if err != nil {
			logrus.Infof("Unable to convert docker container %v to runtime API container: %v", c, err)
			continue
		}

		result = append(result, converted)
	}

	return &v1.ListContainersResponse{Containers: result}, nil
}
