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
	"syscall"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/sirupsen/logrus"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ImageFsInfo returns information of the filesystem of docker data root.
func (ds *dockerService) imageFsInfo() (*runtimeapi.ImageFsInfoResponse, error) {
	// collect info of the filesystem on which docker root resides
	stat := &syscall.Statfs_t{}
	err := syscall.Statfs(ds.dockerRootDir, stat)
	if err != nil {
		logrus.Errorf("Failed to get filesystem info for %s: %v", ds.dockerRootDir, err)
		return nil, err
	}
	usedBytes := (stat.Blocks - stat.Bfree) * uint64(stat.Bsize)
	iNodesUsed := stat.Files - stat.Ffree
	logrus.Debugf("Filesystem usage containing '%s': usedBytes=%v, iNodesUsed=%v", ds.dockerRootDir, usedBytes, iNodesUsed)

	// compute total used bytes by docker images
	images, err := ds.client.ListImages(image.ListOptions{All: true, SharedSize: true})
	if err != nil {
		logrus.Errorf("Failed to get image list from docker: %v", err)
		return nil, err
	}
	var totalImageSize uint64
	sharedSizeMap := make(map[int64]struct{})
	for _, i := range images {
		if i.SharedSize == -1 {
			totalImageSize += uint64(i.Size)
		} else { // docker version >= 23.0
			totalImageSize += (uint64(i.Size) - uint64(i.SharedSize))
			sharedSizeMap[i.SharedSize] = struct{}{}
		}
	}
	for k := range sharedSizeMap {
		totalImageSize += uint64(k)
	}
	logrus.Debugf("Total used bytes by docker images: %v", totalImageSize)

	return &runtimeapi.ImageFsInfoResponse{
		ImageFilesystems: []*runtimeapi.FilesystemUsage{
			{
				Timestamp: time.Now().UnixNano(),
				FsId: &runtimeapi.FilesystemIdentifier{
					Mountpoint: ds.dockerRootDir,
				},
				UsedBytes: &runtimeapi.UInt64Value{
					Value: totalImageSize,
				},
				InodesUsed: &runtimeapi.UInt64Value{
					Value: iNodesUsed,
				},
			}},
		ContainerFilesystems: []*runtimeapi.FilesystemUsage{
			{
				Timestamp: time.Now().UnixNano(),
				FsId: &runtimeapi.FilesystemIdentifier{
					Mountpoint: ds.dockerRootDir,
				},
				UsedBytes: &runtimeapi.UInt64Value{
					Value: ds.containerStatsCache.getWriteableLayer(),
				},
			}},
	}, nil
}
