// +build !linux

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

package kubenet

import (
	"fmt"

	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/network"
)

type kubenetNetworkPlugin struct {
	network.NoopNetworkPlugin
}

func NewPlugin(networkPluginDirs []string, cacheDir string) network.NetworkPlugin {
	return &kubenetNetworkPlugin{}
}

func (plugin *kubenetNetworkPlugin) Init(
	host network.Host,
	hairpinMode config.HairpinMode,
	nonMasqueradeCIDR string,
	mtu int,
) error {
	return fmt.Errorf("Kubenet is not supported in this build")
}

func (plugin *kubenetNetworkPlugin) Name() string {
	return "kubenet"
}

func (plugin *kubenetNetworkPlugin) SetUpPod(
	namespace string,
	name string,
	id config.ContainerID,
	annotations, options map[string]string,
) error {
	return fmt.Errorf("Kubenet is not supported in this build")
}

func (plugin *kubenetNetworkPlugin) TearDownPod(
	namespace string,
	name string,
	id config.ContainerID,
) error {
	return fmt.Errorf("Kubenet is not supported in this build")
}

func (plugin *kubenetNetworkPlugin) GetPodNetworkStatus(
	namespace string,
	name string,
	id config.ContainerID,
) (*network.PodNetworkStatus, error) {
	return nil, fmt.Errorf("Kubenet is not supported in this build")
}
