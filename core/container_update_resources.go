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
	"github.com/docker/docker/api/types/container"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func (ds *dockerService) UpdateContainerResources(
	_ context.Context,
	r *v1.UpdateContainerResourcesRequest,
) (*v1.UpdateContainerResourcesResponse, error) {
	resources := r.Linux
	updateConfig := container.UpdateConfig{
		Resources: container.Resources{
			CPUPeriod:  resources.CpuPeriod,
			CPUQuota:   resources.CpuQuota,
			CPUShares:  resources.CpuShares,
			Memory:     resources.MemoryLimitInBytes,
			MemorySwap: resources.MemoryLimitInBytes,
			CpusetCpus: resources.CpusetCpus,
			CpusetMems: resources.CpusetMems,
		},
	}

	err := ds.client.UpdateContainerResources(r.ContainerId, updateConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to update container %q: %v", r.ContainerId, err)
	}
	return &v1.UpdateContainerResourcesResponse{}, nil
}
