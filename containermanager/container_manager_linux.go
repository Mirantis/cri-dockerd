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

package containermanager

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strconv"
	"time"

	"github.com/opencontainers/runc/libcontainer/cgroups"
	cgroupfs "github.com/opencontainers/runc/libcontainer/cgroups/fs"
	cgroupfs2 "github.com/opencontainers/runc/libcontainer/cgroups/fs2"
	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runc/libcontainer/devices"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	utilversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

const (
	// The percent of the machine memory capacity. The value is used to calculate
	// docker memory resource container's hardlimit to workaround docker memory
	// leakage issue. Please see kubernetes/issues/9881 for more detail.
	dockerMemoryLimitThresholdPercent = 70

	// The minimum memory limit allocated to docker container: 150Mi
	minDockerMemoryLimit = 150 * 1024 * 1024

	// The OOM score adjustment for the docker process (i.e. the docker
	// daemon). Essentially, makes docker very unlikely to experience an oom
	// kill.
	dockerOOMScoreAdj = -999
)

var (
	memoryCapacityRegexp = regexp.MustCompile(`MemTotal:\s*([0-9]+) kB`)
)

// NewContainerManager creates a new instance of ContainerManager
func NewContainerManager(cgroupsName string, client libdocker.DockerClientInterface) ContainerManager {
	return &containerManager{
		cgroupsName: cgroupsName,
		client:      client,
	}
}

type containerManager struct {
	// Docker client.
	client libdocker.DockerClientInterface
	// Name of the cgroups.
	cgroupsName string
	// Manager for the cgroups.
	cgroupsManager cgroups.Manager
}

func (m *containerManager) Start() error {
	if len(m.cgroupsName) != 0 {
		manager, err := createCgroupManager(m.cgroupsName)
		if err != nil {
			return errors.Wrapf(err, "failed to create cgroup manager %s", m.cgroupsName)
		}
		m.cgroupsManager = manager
	}
	go wait.Until(m.doWork, 5*time.Minute, wait.NeverStop)
	return nil
}

func (m *containerManager) doWork() {
	v, err := m.client.Version()
	if err != nil {
		logrus.Errorf("Unable to get docker version: %v", err)
		return
	}
	version, err := utilversion.ParseGeneric(v.APIVersion)
	if err != nil {
		logrus.Errorf("Unable to parse docker version %v: %v", v.APIVersion, err)
		return
	}
	// EnsureDockerInContainer does two things.
	//   1. Ensure processes run in the cgroups if m.cgroupsManager is not nil.
	//   2. Ensure processes have the OOM score applied.
	if err := m.ensureDockerInContainer(version, dockerOOMScoreAdj, m.cgroupsManager); err != nil {
		logrus.Errorf("Unable to ensure the docker processes run in the desired containers: %v", err)
	}
}

func createCgroupManager(name string) (cgroups.Manager, error) {
	var memoryLimit uint64

	memoryCapacity, err := getMemoryCapacity()
	if err != nil {
		logrus.Errorf("Failed to get the memory capacity on machine: %v", err)
	} else {
		memoryLimit = memoryCapacity * dockerMemoryLimitThresholdPercent / 100
	}

	if err != nil || memoryLimit < minDockerMemoryLimit {
		memoryLimit = minDockerMemoryLimit
	}
	logrus.Infof("Configuring resource-only container %s with memory limit %d", name, memoryLimit)

	cg := &configs.Cgroup{
		Parent: "/",
		Name:   name,
		Resources: &configs.Resources{
			Memory:      int64(memoryLimit),
			MemorySwap:  -1,
			SkipDevices: true,
			Devices: []*devices.Rule{
				{
					Minor:       devices.Wildcard,
					Major:       devices.Wildcard,
					Type:        'a',
					Permissions: "rwm",
					Allow:       true,
				},
			},
		},
	}
	if cgroups.IsCgroup2UnifiedMode() {
		return cgroupfs2.NewManager(cg, "")
	}
	return cgroupfs.NewManager(cg, nil)
}

// getMemoryCapacity returns the memory capacity on the machine in bytes.
func getMemoryCapacity() (uint64, error) {
	out, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	return parseCapacity(out, memoryCapacityRegexp)
}

// parseCapacity matches a Regexp in a []byte, returning the resulting value in bytes.
// Assumes that the value matched by the Regexp is in KB.
func parseCapacity(b []byte, r *regexp.Regexp) (uint64, error) {
	matches := r.FindSubmatch(b)
	if len(matches) != 2 {
		return 0, fmt.Errorf("failed to match regexp in output: %q", string(b))
	}
	m, err := strconv.ParseUint(string(matches[1]), 10, 64)
	if err != nil {
		return 0, err
	}

	// Convert to bytes.
	return m * 1024, err
}
