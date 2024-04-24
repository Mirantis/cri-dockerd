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

	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// PodSandboxStatus returns the status of the PodSandbox.
func (ds *dockerService) PodSandboxStatus(
	ctx context.Context,
	req *v1.PodSandboxStatusRequest,
) (*v1.PodSandboxStatusResponse, error) {
	podSandboxID := req.PodSandboxId

	r, metadata, err := ds.getPodSandboxDetails(podSandboxID)
	if err != nil {
		return nil, err
	}

	// Parse the timestamps.
	createdAt, _, _, err := getContainerTimestamps(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp for container %q: %v", podSandboxID, err)
	}
	ct := createdAt.UnixNano()

	// Translate container to sandbox state.
	state := v1.PodSandboxState_SANDBOX_NOTREADY
	if r.State.Running {
		state = v1.PodSandboxState_SANDBOX_READY
	}

	var ips []string
	// This is a workaround for windows, where sandbox is not in use, and pod IP is determined through containers belonging to the Pod.
	if ips = ds.determinePodIPBySandboxID(podSandboxID); len(ips) == 0 {
		ips = ds.getIPs(podSandboxID, r)
	}

	// ip is primary ips
	// ips is all other ips
	ip := ""
	if len(ips) != 0 {
		ip = ips[0]
		ips = ips[1:]
	}

	labels, annotations := extractLabels(r.Config.Labels)
	status := &v1.PodSandboxStatus{
		Id:          r.ID,
		State:       state,
		CreatedAt:   ct,
		Metadata:    metadata,
		Labels:      labels,
		Annotations: annotations,
		Network: &v1.PodSandboxNetworkStatus{
			Ip: ip,
		},
		Linux: &v1.LinuxPodSandboxStatus{
			Namespaces: &v1.Namespace{
				Options: &v1.NamespaceOption{
					Network: networkNamespaceMode(r),
					Pid:     pidNamespaceMode(r),
					Ipc:     ipcNamespaceMode(r),
				},
			},
		},
		RuntimeHandler: r.HostConfig.Runtime,
	}
	// add additional IPs
	additionalPodIPs := make([]*v1.PodIP, 0, len(ips))
	for _, ip := range ips {
		additionalPodIPs = append(additionalPodIPs, &v1.PodIP{
			Ip: ip,
		})
	}
	status.Network.AdditionalIps = additionalPodIPs
	return &v1.PodSandboxStatusResponse{Status: status}, nil
}
