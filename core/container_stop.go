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
	"time"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// StopContainer stops a running container with a grace period (i.e., timeout).
func (ds *dockerService) StopContainer(
	_ context.Context,
	r *v1.StopContainerRequest,
) (*v1.StopContainerResponse, error) {
	err := ds.client.StopContainer(r.ContainerId, time.Duration(r.Timeout)*time.Second)
	if err != nil {
		if libdocker.IsContainerNotFoundError(err) {
			err = status.Error(codes.NotFound, err.Error())
		}
		return nil, err
	}
	return &v1.StopContainerResponse{}, nil
}
