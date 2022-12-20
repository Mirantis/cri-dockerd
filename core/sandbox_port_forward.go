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
	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/network/hostport"
	"github.com/Mirantis/cri-dockerd/store"
	"github.com/Mirantis/cri-dockerd/streaming"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// portMappingGetter is a wrapper around the dockerService that implements
// the network.PortMappingGetter interface.
type portMappingGetter struct {
	ds *dockerService
}

// namespaceGetter is a wrapper around the dockerService that implements
// the network.NamespaceGetter interface.
type namespaceGetter struct {
	ds *dockerService
}

func (n *namespaceGetter) GetNetNS(containerID string) (string, error) {
	return n.ds.GetNetNS(containerID)
}

func (p *portMappingGetter) GetPodPortMappings(
	containerID string,
) ([]*hostport.PortMapping, error) {
	return p.ds.GetPodPortMappings(containerID)
}

// dockerNetworkHost implements network.Host by wrapping the legacy host passed in by the kubelet
// and dockerServices which implements the rest of the network host interfaces.
// The legacy host methods are slated for deletion.
type dockerNetworkHost struct {
	*namespaceGetter
	*portMappingGetter
}

// effectiveHairpinMode determines the effective hairpin mode given the
// configured mode, and whether cbr0 should be configured.
func effectiveHairpinMode(s *config.NetworkPluginSettings) error {
	// The hairpin mode setting doesn't matter if:
	// - We're not using a bridge network. This is hard to check because we might
	//   be using a plugin.
	// - It's set to hairpin-veth for a container runtime that doesn't know how
	//   to set the hairpin flag on the veth's of containers. Currently the
	//   docker runtime is the only one that understands this.
	// - It's set to "none".
	if s.HairpinMode == config.PromiscuousBridge ||
		s.HairpinMode == config.HairpinVeth {
		if s.HairpinMode == config.PromiscuousBridge && s.PluginName != "kubenet" {
			// This is not a valid combination, since promiscuous-bridge only works on kubenet. Users might be using the
			// default values (from before the hairpin-mode flag existed) and we
			// should keep the old behavior.
			logrus.Info(
				"Hairpin mode is set but kubenet is not enabled, falling back to HairpinVeth",
				"hairpinMode",
				s.HairpinMode,
			)
			s.HairpinMode = config.HairpinVeth
			return nil
		}
	} else if s.HairpinMode != config.HairpinNone {
		return fmt.Errorf("unknown value: %q", s.HairpinMode)
	}
	return nil
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox, and returns the address.
func (ds *dockerService) PortForward(
	_ context.Context,
	req *v1.PortForwardRequest,
) (*v1.PortForwardResponse, error) {
	if ds.streamingServer == nil {
		return nil, streaming.NewErrorStreamingDisabled("port forward")
	}
	_, err := libdocker.CheckContainerStatus(ds.client, req.PodSandboxId)
	if err != nil {
		return nil, err
	}
	return ds.streamingServer.GetPortForward(req)
}

// GetNetNS returns the network namespace of the given containerID. The ID
// supplied is typically the ID of a pod sandbox. This getter doesn't try
// to map non-sandbox IDs to their respective sandboxes.
func (ds *dockerService) GetNetNS(podSandboxID string) (string, error) {
	r, err := ds.client.InspectContainer(podSandboxID)
	if err != nil {
		return "", err
	}
	return getNetworkNamespace(r)
}

// GetPodPortMappings returns the port mappings of the given podSandbox ID.
func (ds *dockerService) GetPodPortMappings(podSandboxID string) ([]*hostport.PortMapping, error) {
	checkpoint := NewPodSandboxCheckpoint("", "", &CheckpointData{})
	err := ds.checkpointManager.GetCheckpoint(podSandboxID, checkpoint)
	// Return empty portMappings if checkpoint is not found
	if err != nil {
		if err == store.ErrCheckpointNotFound {
			return nil, nil
		}
		errRem := ds.checkpointManager.RemoveCheckpoint(podSandboxID)
		if errRem != nil {
			logrus.Error(
				errRem,
				"Failed to delete corrupt checkpoint for sandbox",
				"podSandboxID",
				podSandboxID,
			)
		}
		return nil, err
	}
	_, _, _, checkpointedPortMappings, _ := checkpoint.GetData()
	portMappings := make([]*hostport.PortMapping, 0, len(checkpointedPortMappings))
	for _, pm := range checkpointedPortMappings {
		portMappings = append(portMappings, &hostport.PortMapping{
			HostPort:      *pm.HostPort,
			ContainerPort: *pm.ContainerPort,
			Protocol:      *pm.Protocol,
			HostIP:        pm.HostIP,
		})
	}
	return portMappings, nil
}
