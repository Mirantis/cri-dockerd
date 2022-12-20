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
	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ContainerStatus inspects the docker container and returns the status.
func (ds *dockerService) ContainerStatus(
	_ context.Context,
	req *v1.ContainerStatusRequest,
) (*v1.ContainerStatusResponse, error) {
	containerID := req.ContainerId
	r, err := ds.client.InspectContainer(containerID)
	if err != nil {
		return nil, err
	}

	// Parse the timestamps.
	createdAt, startedAt, finishedAt, err := getContainerTimestamps(r)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp for container %q: %v", containerID, err)
	}

	// Convert the image id to a pullable id.
	ir, err := ds.client.InspectImageByID(r.Image)
	if err != nil {
		if !libdocker.IsImageNotFoundError(err) {
			return nil, fmt.Errorf(
				"unable to inspect docker image %q while inspecting docker container %q: %v",
				r.Image,
				containerID,
				err,
			)
		}
		logrus.Debugf("Image %s not found while inspecting docker container %s: %v", r.Image, containerID, err)
	}
	imageID := toPullableImageID(r.Image, ir)

	// Convert the mounts.
	mounts := make([]*v1.Mount, 0, len(r.Mounts))
	for i := range r.Mounts {
		m := r.Mounts[i]
		readonly := !m.RW
		mounts = append(mounts, &v1.Mount{
			HostPath:      m.Source,
			ContainerPath: m.Destination,
			Readonly:      readonly,
			// Note: Can't set SeLinuxRelabel
		})
	}
	// Interpret container states.
	var state v1.ContainerState
	var reason, message string
	if r.State.Running {
		// Container is running.
		state = v1.ContainerState_CONTAINER_RUNNING
	} else {
		// Container is *not* running. We need to get more details.
		//    * Case 1: container has run and exited with non-zero finishedAt
		//              time.
		//    * Case 2: container has failed to start; it has a zero finishedAt
		//              time, but a non-zero exit code.
		//    * Case 3: container has been created, but not started (yet).
		if !finishedAt.IsZero() { // Case 1
			state = v1.ContainerState_CONTAINER_EXITED
			switch {
			case r.State.OOMKilled:
				// Note: if an application handles OOMKilled gracefully, the
				// exit code could be zero.
				reason = "OOMKilled"
			case r.State.ExitCode == 0:
				reason = "Completed"
			default:
				reason = "Error"
			}
		} else if r.State.ExitCode != 0 { // Case 2
			state = v1.ContainerState_CONTAINER_EXITED
			// Adjust finshedAt and startedAt time to createdAt time to avoid
			// the confusion.
			finishedAt, startedAt = createdAt, createdAt
			reason = "ContainerCannotRun"
		} else { // Case 3
			state = v1.ContainerState_CONTAINER_CREATED
		}
		message = r.State.Error
	}

	// Convert to unix timestamps.
	ct, st, ft := createdAt.UnixNano(), startedAt.UnixNano(), finishedAt.UnixNano()
	exitCode := int32(r.State.ExitCode)

	metadata, err := parseContainerName(r.Name)
	if err != nil {
		return nil, err
	}

	labels, annotations := extractLabels(r.Config.Labels)
	imageName := r.Config.Image
	if ir != nil && len(ir.RepoTags) > 0 {
		imageName = ir.RepoTags[0]
	}
	status := &v1.ContainerStatus{
		Id:          r.ID,
		Metadata:    metadata,
		Image:       &v1.ImageSpec{Image: imageName},
		ImageRef:    imageID,
		Mounts:      mounts,
		ExitCode:    exitCode,
		State:       state,
		CreatedAt:   ct,
		StartedAt:   st,
		FinishedAt:  ft,
		Reason:      reason,
		Message:     message,
		Labels:      labels,
		Annotations: annotations,
		LogPath:     r.Config.Labels[containerLogPathLabelKey],
	}
	res := v1.ContainerStatusResponse{Status: status}
	if req.GetVerbose() {
		containerInfo, err := containerInspectToRuntimeAPIContainerInfo(r)
		if err != nil {
			return nil, err
		}
		res.Info = containerInfo
	}
	return &res, nil
}
