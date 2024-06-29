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

package libdocker

import (
	"os"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	dockerbackend "github.com/docker/docker/api/types/backend"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimagetypes "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	dockersystem "github.com/docker/docker/api/types/system"
	dockerapi "github.com/docker/docker/client"
	"github.com/sirupsen/logrus"
)

const (
	// https://docs.docker.com/engine/reference/api/docker_remote_api/
	// docker version should be at least 23.0.0.
	// https://github.com/moby/moby/commit/304fbf080465e7097a6ab16b1f2a540d02bc7d75
	MinimumDockerAPIVersion = "1.42.0"

	// Status of a container returned by ListContainers.
	StatusRunningPrefix = "Up"
	StatusCreatedPrefix = "Created"
	StatusExitedPrefix  = "Exited"

	// Fake docker endpoint
	FakeDockerEndpoint = "fake://"
)

// DockerClientInterface is an abstract interface for testability.  It abstracts the interface of docker client.
type DockerClientInterface interface {
	ListContainers(options dockercontainer.ListOptions) ([]dockertypes.Container, error)
	InspectContainer(id string) (*dockertypes.ContainerJSON, error)
	InspectContainerWithSize(id string) (*dockertypes.ContainerJSON, error)
	CreateContainer(
		dockerbackend.ContainerCreateConfig,
	) (*dockercontainer.CreateResponse, error)
	StartContainer(id string) error
	StopContainer(id string, timeout time.Duration) error
	UpdateContainerResources(id string, updateConfig dockercontainer.UpdateConfig) error
	RemoveContainer(id string, opts dockercontainer.RemoveOptions) error
	InspectImageByRef(imageRef string) (*dockertypes.ImageInspect, error)
	InspectImageByID(imageID string) (*dockertypes.ImageInspect, error)
	ListImages(opts dockerimagetypes.ListOptions) ([]dockerimagetypes.Summary, error)
	PullImage(image string, auth dockerregistry.AuthConfig, opts dockerimagetypes.PullOptions) error
	RemoveImage(imageStr string, opts dockerimagetypes.RemoveOptions) ([]dockerimagetypes.DeleteResponse, error)
	ImageHistory(id string) ([]dockerimagetypes.HistoryResponseItem, error)
	Logs(string, dockercontainer.LogsOptions, StreamOptions) error
	Version() (*dockertypes.Version, error)
	Info() (*dockersystem.Info, error)
	CreateExec(string, dockercontainer.ExecOptions) (*dockertypes.IDResponse, error)
	StartExec(string, dockercontainer.ExecStartOptions, StreamOptions) error
	InspectExec(id string) (*dockercontainer.ExecInspect, error)
	AttachToContainer(string, dockercontainer.AttachOptions, StreamOptions) error
	ResizeContainerTTY(id string, height, width uint) error
	ResizeExecTTY(id string, height, width uint) error
	GetContainerStats(id string) (*dockercontainer.StatsResponse, error)
}

// Get a *dockerapi.Client, either using the endpoint passed in, or using
// DOCKER_HOST, DOCKER_TLS_VERIFY, and DOCKER_CERT path per their spec
func getDockerClient(dockerEndpoint string) (*dockerapi.Client, error) {
	if len(dockerEndpoint) > 0 {
		logrus.Infof("Connecting to docker on the Endpoint %s", dockerEndpoint)
		return dockerapi.NewClientWithOpts(
			dockerapi.WithHost(dockerEndpoint),
			dockerapi.WithVersion(""),
		)
	}
	return dockerapi.NewClientWithOpts(dockerapi.FromEnv)
}

// ConnectToDockerOrDie creates docker client connecting to docker daemon.
// If the endpoint passed in is "fake://", a fake docker client
// will be returned. The program exits if error occurs. The requestTimeout
// is the timeout for docker requests. If timeout is exceeded, the request
// will be cancelled and throw out an error. If requestTimeout is 0, a default
// value will be applied.
func ConnectToDockerOrDie(
	dockerEndpoint string,
	requestTimeout, imagePullProgressDeadline time.Duration,
) DockerClientInterface {
	client, err := getDockerClient(dockerEndpoint)
	if err != nil {
		logrus.Errorf("Couldn't connect to docker: %v", err)
		os.Exit(1)

	}
	logrus.Infof("Start docker client with request timeout %s", requestTimeout)
	return newKubeDockerClient(client, requestTimeout, imagePullProgressDeadline)
}
