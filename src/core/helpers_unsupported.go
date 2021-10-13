// +build !linux,!windows

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
	"fmt"

	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// DefaultMemorySwap always returns -1 for no memory swap in a sandbox
func DefaultMemorySwap() int64 {
	return -1
}

func (ds *core.dockerService) getSecurityOpts(
	seccompProfile string,
	separator rune,
) ([]string, error) {
	logrus.Info("getSecurityOpts is unsupported in this build")
	return nil, nil
}

func (ds *core.dockerService) getSandBoxSecurityOpts(separator rune) []string {
	logrus.Info("getSandBoxSecurityOpts is unsupported in this build")
	return nil
}

func (ds *core.dockerService) updateCreateConfig(
	createConfig *dockertypes.ContainerCreateConfig,
	config *runtimeapi.ContainerConfig,
	sandboxConfig *runtimeapi.PodSandboxConfig,
	podSandboxID string, securityOptSep rune, apiVersion *semver.Version) error {
	logrus.Info("updateCreateConfig is unsupported in this build")
	return nil
}

func (ds *core.dockerService) determinePodIPBySandboxID(uid string) []string {
	logrus.Info("determinePodIPBySandboxID is unsupported in this build")
	return nil
}

func getNetworkNamespace(c *dockertypes.ContainerJSON) (string, error) {
	return "", fmt.Errorf("unsupported platform")
}
