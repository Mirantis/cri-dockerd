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
	"time"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func (ds *dockerService) getContainerStats(container *runtimeapi.Container) (*runtimeapi.ContainerStats, error) {
	containerID := container.Id
	statsJSON, err := ds.client.GetContainerStats(containerID)
	if err != nil {
		return nil, err
	}

	dockerStats := statsJSON.Stats
	timestamp := time.Now().UnixNano()
	containerStats := &runtimeapi.ContainerStats{
		Attributes: &runtimeapi.ContainerAttributes{
			Id:          containerID,
			Metadata:    container.Metadata,
			Labels:      container.Labels,
			Annotations: container.Annotations,
		},
		Cpu: &runtimeapi.CpuUsage{
			Timestamp: timestamp,
			UsageCoreNanoSeconds: &runtimeapi.UInt64Value{
				Value: dockerStats.CPUStats.CPUUsage.TotalUsage,
			},
		},
		Memory: &runtimeapi.MemoryUsage{
			Timestamp: timestamp,
			WorkingSetBytes: &runtimeapi.UInt64Value{
				Value: dockerStats.MemoryStats.Usage,
			},
		},
	}

	cstat := ds.containerStatsCache.getStats(containerID)
	if cstat != nil && cstat.isInitialized() {
		containerStats.WritableLayer = &runtimeapi.FilesystemUsage{
			Timestamp: timestamp,
			FsId:      &runtimeapi.FilesystemIdentifier{Mountpoint: ds.dockerRootDir},
			UsedBytes: &runtimeapi.UInt64Value{Value: cstat.getContainerRWSize()},
		}
	}
	return containerStats, nil
}
