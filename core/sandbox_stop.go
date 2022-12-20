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
	"github.com/Mirantis/cri-dockerd/store"
	"github.com/Mirantis/cri-dockerd/utils/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be force terminated.
// better to cut our losses assuming an out of band GC routine will cleanup
// after us?
func (ds *dockerService) StopPodSandbox(
	ctx context.Context,
	r *v1.StopPodSandboxRequest,
) (*v1.StopPodSandboxResponse, error) {
	var namespace, name string
	var hostNetwork bool

	podSandboxID := r.PodSandboxId
	resp := &v1.StopPodSandboxResponse{}

	// Try to retrieve minimal sandbox information from docker daemon or sandbox checkpoint.
	inspectResult, metadata, statusErr := ds.getPodSandboxDetails(podSandboxID)
	if statusErr == nil {
		namespace = metadata.Namespace
		name = metadata.Name
		hostNetwork = (networkNamespaceMode(inspectResult) == v1.NamespaceMode_NODE)
	} else {
		checkpoint := NewPodSandboxCheckpoint("", "", &CheckpointData{})
		checkpointErr := ds.checkpointManager.GetCheckpoint(podSandboxID, checkpoint)

		// Proceed if both sandbox container and checkpoint could not be found. This means that following
		// actions will only have sandbox ID and not have pod namespace and name information.
		// Return error if encounter any unexpected error.
		if checkpointErr != nil {
			if checkpointErr != store.ErrCheckpointNotFound {
				err := ds.checkpointManager.RemoveCheckpoint(podSandboxID)
				if err != nil {
					logrus.Errorf("Failed to delete corrupt checkpoint for sandbox %s: %v", podSandboxID, err)
				}
			}
			if libdocker.IsContainerNotFoundError(statusErr) {
				logrus.Infof(
					"Both sandbox container and checkpoint could not be found with id %q. "+
						"Proceed without further sandbox information.", podSandboxID)
			} else {
				return nil, errors.NewAggregate([]error{
					fmt.Errorf("failed to get checkpoint for sandbox %q: %v", podSandboxID, checkpointErr),
					fmt.Errorf("failed to get sandbox status: %v", statusErr)})
			}
		} else {
			_, name, namespace, _, hostNetwork = checkpoint.GetData()
		}
	}

	// WARNING: The following operations made the following assumption:
	// 1. kubelet will retry on any error returned by StopPodSandbox.
	// 2. tearing down network and stopping sandbox container can succeed in any sequence.
	// This depends on the implementation detail of network plugin and proper error handling.
	// For kubenet, if tearing down network failed and sandbox container is stopped, kubelet
	// will retry. On retry, kubenet will not be able to retrieve network namespace of the sandbox
	// since it is stopped. With empty network namespace, CNI bridge plugin will conduct best
	// effort clean up and will not return error.
	errList := []error{}
	ready, ok := ds.getNetworkReady(podSandboxID)
	if !hostNetwork && (ready || !ok) {
		// Only tear down the pod network if we haven't done so already
		cID := config.BuildContainerID(runtimeName, podSandboxID)
		err := ds.network.TearDownPod(namespace, name, cID)
		if err == nil {
			ds.setNetworkReady(podSandboxID, false)
		} else {
			errList = append(errList, err)
		}
	}
	if err := ds.client.StopContainer(podSandboxID, defaultSandboxGracePeriod); err != nil {
		// Do not return error if the container does not exist
		if !libdocker.IsContainerNotFoundError(err) {
			logrus.Errorf("Failed to stop sandbox %s: %v", podSandboxID, err)
			errList = append(errList, err)
		} else {
			// remove the checkpoint for any sandbox that is not found in the runtime
			ds.checkpointManager.RemoveCheckpoint(podSandboxID)
		}
	}

	if len(errList) == 0 {
		return resp, nil
	}

	return nil, errors.NewAggregate(errList)
}
