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
	"strings"
	"time"

	"github.com/Microsoft/hcsshim"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func (ds *dockerService) getContainerStats(container *runtimeapi.Container) (*runtimeapi.ContainerStats, error) {
	containerID := container.Id
	hcsshimContainer, err := hcsshim.OpenContainer(containerID)
	if err != nil {
		// As we moved from using Docker stats to hcsshim directly, we may query HCS with already exited container IDs.
		// That will typically happen with init-containers in Exited state. Docker still knows about them but the HCS does not.
		// As we don't want to block stats retrieval for other containers, we only log errors.
		if !hcsshim.IsNotExist(err) && !hcsshim.IsAlreadyStopped(err) {
			logrus.Info("Error opening container for ID: %d (stats will be missing): %v", containerID, err)
		}
		return nil, nil
	}
	defer func() {
		closeErr := hcsshimContainer.Close()
		if closeErr != nil {
			logrus.Errorf("Error closing container %d: %v", containerID, err)
		}
	}()

	stats, err := hcsshimContainer.Statistics()
	if err != nil {
		if strings.Contains(err.Error(), "0x5") || strings.Contains(err.Error(), "0xc0370105") {
			// When the container is just created, querying for stats causes access errors because it hasn't started yet
			// This is transient; skip container for now
			//
			// These hcs errors do not have helpers exposed in public package so need to query for the known codes
			// https://github.com/microsoft/hcsshim/blob/master/internal/hcs/errors.go
			// PR to expose helpers in hcsshim: https://github.com/microsoft/hcsshim/pull/933
			logrus.Info(
				"Container %dis not in a state that stats can be accessed. This occurs when the container is created but not started: %v",
				containerID,
				err,
			)
			return nil, nil
		}
		return nil, err
	}

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
			// have to multiply cpu usage by 100 since stats units is in 100's of nano seconds for Windows
			UsageCoreNanoSeconds: &runtimeapi.UInt64Value{
				Value: stats.Processor.TotalRuntime100ns * 100,
			},
		},
		Memory: &runtimeapi.MemoryUsage{
			Timestamp: timestamp,
			WorkingSetBytes: &runtimeapi.UInt64Value{
				Value: stats.Memory.UsagePrivateWorkingSetBytes,
			},
		},
		WritableLayer: &runtimeapi.FilesystemUsage{
			Timestamp: timestamp,
			FsId:      &runtimeapi.FilesystemIdentifier{Mountpoint: ds.dockerRootDir},
			UsedBytes: &runtimeapi.UInt64Value{Value: 0},
		},
	}
	return containerStats, nil
}
