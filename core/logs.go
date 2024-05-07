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
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/Mirantis/cri-dockerd/config"

	"github.com/armon/circbuf"
	dockercontainer "github.com/docker/docker/api/types/container"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

// ReopenContainerLog reopens the container log file.
func (ds *dockerService) ReopenContainerLog(
	_ context.Context,
	_ *runtimeapi.ReopenContainerLogRequest,
) (*runtimeapi.ReopenContainerLogResponse, error) {
	return nil, fmt.Errorf("docker does not support reopening container log files")
}

// GetContainerLogs get container logs directly from docker daemon.
func (ds *dockerService) GetContainerLogs(
	_ context.Context,
	pod *v1.Pod,
	containerID config.ContainerID,
	logOptions *v1.PodLogOptions,
	stdout, stderr io.Writer,
) error {
	container, err := ds.client.InspectContainer(containerID.ID)
	if err != nil {
		return err
	}

	var since int64
	if logOptions.SinceSeconds != nil {
		t := metav1.Now().Add(-time.Duration(*logOptions.SinceSeconds) * time.Second)
		since = t.Unix()
	}
	if logOptions.SinceTime != nil {
		since = logOptions.SinceTime.Unix()
	}
	opts := dockercontainer.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      strconv.FormatInt(since, 10),
		Timestamps: logOptions.Timestamps,
		Follow:     logOptions.Follow,
	}
	if logOptions.TailLines != nil {
		opts.Tail = strconv.FormatInt(*logOptions.TailLines, 10)
	}

	if logOptions.LimitBytes != nil {
		// stdout and stderr share the total write limit
		max := *logOptions.LimitBytes
		stderr = SharedLimitWriter(stderr, &max)
		stdout = SharedLimitWriter(stdout, &max)
	}
	sopts := libdocker.StreamOptions{
		OutputStream: stdout,
		ErrorStream:  stderr,
		RawTerminal:  container.Config.Tty,
	}
	err = ds.client.Logs(containerID.ID, opts, sopts)
	if errors.Is(err, errMaximumWrite) {
		logrus.Debugf("Finished logs, hit byte limit: %d", *logOptions.LimitBytes)
		err = nil
	}
	return err
}

// GetContainerLogTail attempts to read up to MaxContainerTerminationMessageLogLength
// from the end of the log when docker is configured with a log driver other than json-log.
// It reads up to MaxContainerTerminationMessageLogLines lines.
func (ds *dockerService) GetContainerLogTail(
	uid config.UID,
	name, namespace string,
	containerID config.ContainerID,
) (string, error) {
	value := int64(config.MaxContainerTerminationMessageLogLines)
	buf, _ := circbuf.NewBuffer(config.MaxContainerTerminationMessageLogLength)
	// Although this is not a full spec pod, dockerLegacyService.GetContainerLogs() currently completely ignores its pod param
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       kubetypes.UID(uid),
			Name:      name,
			Namespace: namespace,
		},
	}
	err := ds.GetContainerLogs(
		context.Background(),
		pod,
		containerID,
		&v1.PodLogOptions{TailLines: &value},
		buf,
		buf,
	)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// criSupportedLogDrivers are log drivers supported by native CRI integration.
var criSupportedLogDrivers = []string{"json-file"}

// IsCRISupportedLogDriver checks whether the logging driver used by docker is
// supported by native CRI integration.
func (ds *dockerService) IsCRISupportedLogDriver() (bool, error) {
	info, err := ds.getDockerInfo()
	if err != nil {
		return false, fmt.Errorf("failed to get docker info: %v", err)
	}
	for _, driver := range criSupportedLogDrivers {
		if info.LoggingDriver == driver {
			return true, nil
		}
	}
	return false, nil
}

// getContainerLogPath returns the container log path specified by kubelet and the real
// path where docker stores the container log.
func (ds *dockerService) getContainerLogPath(containerID string) (string, string, error) {
	info, err := ds.client.InspectContainer(containerID)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect container %q: %v", containerID, err)
	}
	return info.Config.Labels[containerLogPathLabelKey], info.LogPath, nil
}

// createContainerLogSymlink creates the symlink for docker container log.
func (ds *dockerService) createContainerLogSymlink(containerID string) error {
	path, realPath, err := ds.getContainerLogPath(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container %q log path: %v", containerID, err)
	}

	if path == "" {
		logrus.Debugf("Container log path for Container ID %s isn't specified, will not create symlink", containerID)
		return nil
	}

	if realPath != "" {
		// Only create the symlink when container log path is specified and log file exists.
		// Delete possibly existing file first
		if err = ds.os.Remove(path); err == nil {
			logrus.Debugf("Deleted previously existing symlink file: %s", path)
		}
		if err = ds.os.Symlink(realPath, path); err != nil {
			return fmt.Errorf(
				"failed to create symbolic link %q to the container log file %q for container %q: %v",
				path,
				realPath,
				containerID,
				err,
			)
		}
	} else {
		supported, err := ds.IsCRISupportedLogDriver()
		if err != nil {
			logrus.Errorf("Failed to check supported logging driver for CRI: %v", err)
			return nil
		}

		if supported {
			logrus.Info("Cannot create symbolic link because container log file doesn't exist!")
		} else {
			logrus.Debug("Unsupported logging driver by CRI")
		}
	}

	return nil
}

// removeContainerLogSymlink removes the symlink for docker container log.
func (ds *dockerService) removeContainerLogSymlink(containerID string) error {
	path, _, err := ds.getContainerLogPath(containerID)
	if err != nil {
		return fmt.Errorf("failed to get container %q log path: %v", containerID, err)
	}
	if path != "" {
		// Only remove the symlink when container log path is specified.
		err := ds.os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf(
				"failed to remove container %q log symlink %q: %v",
				containerID,
				path,
				err,
			)
		}
	}
	return nil
}
