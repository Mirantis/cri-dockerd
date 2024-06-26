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
	"encoding/json"
	"errors"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Mirantis/cri-dockerd/store"

	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	dockersystem "github.com/docker/docker/api/types/system"
	"github.com/golang/mock/gomock"
	ociruntimefeatures "github.com/opencontainers/runtime-spec/specs-go/features"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	containertest "k8s.io/kubernetes/pkg/kubelet/container/testing"
	clock "k8s.io/utils/clock/testing"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/network"
	nettest "github.com/Mirantis/cri-dockerd/network/testing"
)

// newTestNetworkPlugin returns a mock plugin that implements network.NetworkPlugin
func newTestNetworkPlugin(t *testing.T) *nettest.MockNetworkPlugin {
	ctrl := gomock.NewController(t)
	return nettest.NewMockNetworkPlugin(ctrl)
}

type mockCheckpointManager struct {
	lock       sync.Mutex
	checkpoint map[string]*PodSandboxCheckpoint
}

func (ckm *mockCheckpointManager) CreateCheckpoint(
	checkpointKey string,
	checkpoint store.Checkpoint,
) error {
	ckm.lock.Lock()
	ckm.checkpoint[checkpointKey] = checkpoint.(*PodSandboxCheckpoint)
	ckm.lock.Unlock()
	return nil
}

func (ckm *mockCheckpointManager) GetCheckpoint(
	checkpointKey string,
	checkpoint store.Checkpoint,
) error {
	ckm.lock.Lock()
	defer ckm.lock.Unlock()
	*(checkpoint.(*PodSandboxCheckpoint)) = *(ckm.checkpoint[checkpointKey])
	return nil
}

func (ckm *mockCheckpointManager) RemoveCheckpoint(checkpointKey string) error {
	ckm.lock.Lock()
	defer ckm.lock.Unlock()
	_, ok := ckm.checkpoint[checkpointKey]
	if ok {
		delete(ckm.checkpoint, "moo")
	}
	return nil
}

func (ckm *mockCheckpointManager) ListCheckpoints() ([]string, error) {
	var keys []string
	ckm.lock.Lock()
	defer ckm.lock.Unlock()
	for key := range ckm.checkpoint {
		keys = append(keys, key)
	}
	return keys, nil
}

func newMockCheckpointManager() store.CheckpointManager {
	return &mockCheckpointManager{
		checkpoint: make(map[string]*PodSandboxCheckpoint),
		lock:       sync.Mutex{},
	}
}

func newTestDockerService() (*dockerService, *libdocker.FakeDockerClient, *clock.FakeClock) {
	fakeClock := clock.NewFakeClock(time.Time{})
	c := libdocker.NewFakeDockerClient().WithClock(
		fakeClock,
	).WithVersion(
		"1.11.2",
		"1.23",
	).WithRandSource(
		rand.NewSource(0),
	)
	pm := network.NewPluginManager(&network.NoopNetworkPlugin{})
	ckm := newMockCheckpointManager()
	return &dockerService{
		client:              c,
		os:                  &containertest.FakeOS{},
		network:             pm,
		checkpointManager:   ckm,
		networkReady:        make(map[string]bool),
		dockerRootDir:       "/docker/root/dir",
		containerStatsCache: newContainerStatsCache(),
	}, c, fakeClock
}

func newTestDockerServiceWithVersionCache() (*dockerService, *libdocker.FakeDockerClient, *clock.FakeClock) {
	ds, c, fakeClock := newTestDockerService()
	return ds, c, fakeClock
}

// TestStatus tests the runtime status logic.
func TestStatus(t *testing.T) {
	ds, fDocker, _ := newTestDockerService()

	assertStatus := func(expected map[string]bool, status *runtimeapi.RuntimeStatus) {
		conditions := status.GetConditions()
		assert.Equal(t, len(expected), len(conditions))
		for k, v := range expected {
			for _, c := range conditions {
				if k == c.Type {
					assert.Equal(t, v, c.Status)
				}
			}
		}
	}

	// Should report ready status if version returns no error.
	statusResp, err := ds.Status(getTestCTX(), &runtimeapi.StatusRequest{})
	require.NoError(t, err)
	assertStatus(map[string]bool{
		runtimeapi.RuntimeReady: true,
		runtimeapi.NetworkReady: true,
	}, statusResp.Status)

	// Should not report ready status if version returns error.
	fDocker.InjectError("version", errors.New("test error"))
	statusResp, err = ds.Status(getTestCTX(), &runtimeapi.StatusRequest{})
	assert.NoError(t, err)
	assertStatus(map[string]bool{
		runtimeapi.RuntimeReady: true,
		runtimeapi.NetworkReady: true,
	}, statusResp.Status)

	// Should report info if verbose.
	statusResp, err = ds.Status(getTestCTX(), &runtimeapi.StatusRequest{Verbose: true})
	require.NoError(t, err)
	assert.NotNil(t, statusResp.Info)

	// Should not report ready status is network plugin returns error.
	mockPlugin := newTestNetworkPlugin(t)
	ds.network = network.NewPluginManager(mockPlugin)
	defer mockPlugin.Finish()
	mockPlugin.EXPECT().Status().Return(errors.New("network error"))
	statusResp, err = ds.Status(getTestCTX(), &runtimeapi.StatusRequest{})
	assert.NoError(t, err)
	assertStatus(map[string]bool{
		runtimeapi.RuntimeReady: true,
		runtimeapi.NetworkReady: false,
	}, statusResp.Status)
}

// TestRuntimeConfig tests the runtime config logic.
func TestRuntimeConfig(t *testing.T) {
	ds, _, _ := newTestDockerService()
	ds.cgroupDriver = "systemd"

	configResp, err := ds.RuntimeConfig(getTestCTX(), &runtimeapi.RuntimeConfigRequest{})
	require.NoError(t, err)
	if runtime.GOOS == "linux" {
		assert.Equal(t, runtimeapi.CgroupDriver_SYSTEMD, configResp.Linux.CgroupDriver)
	}
}

func TestVersion(t *testing.T) {
	ds, _, _ := newTestDockerService()

	expectedVersion := &dockertypes.Version{Version: "1.11.2", APIVersion: "1.23.0"}
	v, err := ds.getDockerVersion()
	require.NoError(t, err)
	assert.Equal(t, expectedVersion, v)

	expectedAPIVersion := &semver.Version{Major: 1, Minor: 23, Patch: 0}
	apiVersion, err := ds.getDockerAPIVersion()
	require.NoError(t, err)
	assert.Equal(t, expectedAPIVersion, apiVersion)
}

func TestAPIVersionWithCache(t *testing.T) {
	ds, _, _ := newTestDockerServiceWithVersionCache()

	expected := &semver.Version{Major: 1, Minor: 23, Patch: 0}
	version, err := ds.getDockerAPIVersion()
	require.NoError(t, err)
	assert.Equal(t, expected, version)
}

func TestGetRuntimeHandlers(t *testing.T) {
	runcFeatures := ociruntimefeatures.Features{
		MountOptions: []string{"rro"},
	}
	runcFeaturesJSON, err := json.Marshal(runcFeatures)
	assert.NoError(t, err)
	info := &dockersystem.Info{
		Runtimes: map[string]dockersystem.RuntimeWithStatus{
			"io.containerd.runc.v2": dockersystem.RuntimeWithStatus{
				Runtime: dockersystem.Runtime{
					Path: "runc",
				},
				Status: map[string]string{
					"org.opencontainers.runtime-spec.features": string(runcFeaturesJSON),
				},
			},
			"runc": dockersystem.RuntimeWithStatus{
				Runtime: dockersystem.Runtime{
					Path: "runc",
				},
				Status: map[string]string{
					"org.opencontainers.runtime-spec.features": string(runcFeaturesJSON),
				},
			},
			"runsc": dockersystem.RuntimeWithStatus{
				Runtime: dockersystem.Runtime{
					Path: "/usr/local/bin/runsc",
				},
			},
		},
		DefaultRuntime: "runc",
	}

	handlers, err := getRuntimeHandlers(info)
	assert.NoError(t, err)

	expectedHandlers := []runtimeapi.RuntimeHandler{
		{
			Name: "",
			Features: &runtimeapi.RuntimeHandlerFeatures{
				RecursiveReadOnlyMounts: true,
			},
		},
		{
			Name: "io.containerd.runc.v2",
			Features: &runtimeapi.RuntimeHandlerFeatures{
				RecursiveReadOnlyMounts: true,
			},
		},
		{
			Name: "runc",
			Features: &runtimeapi.RuntimeHandlerFeatures{
				RecursiveReadOnlyMounts: true,
			},
		},

		{
			Name: "runsc",
			Features: &runtimeapi.RuntimeHandlerFeatures{
				RecursiveReadOnlyMounts: false,
			},
		},
	}
	for i, f := range handlers {
		assert.Equal(t, expectedHandlers[i].Name, f.Name)
		assert.Equal(t, expectedHandlers[i].Features, f.Features)
		// ignore protobuf fields
	}
}
