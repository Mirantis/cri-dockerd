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
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

func TestContainerStats(t *testing.T) {
	labels := map[string]string{containerTypeLabelKey: containerTypeLabelContainer}
	tests := map[string]struct {
		containerID    string
		container      *libdocker.FakeContainer
		containerStats *container.StatsResponse
		calledDetails  []libdocker.CalledDetail
	}{
		"container exists": {
			"k8s_fake_container",
			&libdocker.FakeContainer{
				ID:   "k8s_fake_container",
				Name: "k8s_fake_container_1_2_1",
				Config: &container.Config{
					Labels: labels,
				},
			},
			&container.StatsResponse{},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("list", nil),
				libdocker.NewCalledDetail("get_container_stats", nil),
			},
		},
		"container doesn't exists": {
			"k8s_nonexistant_fake_container",
			&libdocker.FakeContainer{
				ID:   "k8s_fake_container",
				Name: "k8s_fake_container_1_2_1",
				Config: &container.Config{
					Labels: labels,
				},
			},
			&container.StatsResponse{},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("list", nil),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ds, fakeDocker, _ := newTestDockerService()
			fakeDocker.SetFakeContainers([]*libdocker.FakeContainer{test.container})
			fakeDocker.InjectContainerStats(
				map[string]*container.StatsResponse{test.container.ID: test.containerStats},
			)
			ds.ContainerStats(
				getTestCTX(),
				&runtimeapi.ContainerStatsRequest{ContainerId: test.containerID},
			)
			err := fakeDocker.AssertCallDetails(test.calledDetails...)
			assert.NoError(t, err)
		})
	}
}
