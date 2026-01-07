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
	"fmt"
	"io"
	"net/http"
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
	opts := []dockerapi.Opt{}

	if len(dockerEndpoint) > 0 {
		logrus.Infof("Connecting to docker on the Endpoint %s", dockerEndpoint)
		opts = append(opts, dockerapi.WithHost(dockerEndpoint))
		opts = append(opts, dockerapi.WithVersion(""))
	} else {
		logrus.Info("Connecting to docker using environment configuration")
		opts = append(opts, dockerapi.FromEnv)
	}

	if logrus.GetLevel() >= logrus.DebugLevel {
		opts = append(opts, dockerapi.WithHTTPClient(&http.Client{
			Transport: newDebugTransport(http.DefaultTransport),
		}))
	}

	return dockerapi.NewClientWithOpts(opts...)
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

func newDebugTransport(baseTransport http.RoundTripper) http.RoundTripper {
	return &debugTransport{base: baseTransport}
}

type debugTransport struct {
	base http.RoundTripper
}

func (t *debugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Body = &interceptor{
		ReadCloser: req.Body,
		body:       []byte{},
		complete: func(body []byte) {
			logrus.Debugf("(dockerapi) %s %s: %s", req.Method, req.URL.String(), string(body))
		},
	}
	resp, err := t.base.RoundTrip(req)
	logrus.Debugf("(dockerapi) %s %s: %s", req.Method, req.URL.String(), func() string {
		if err != nil {
			return fmt.Sprintf("error: %v", err)
		}
		return resp.Status
	}())
	return resp, err
}

type interceptor struct {
	io.ReadCloser
	body     []byte
	complete func(body []byte)
}

func (p *interceptor) Read(b []byte) (int, error) {
	if p.ReadCloser == nil {
		return 0, io.EOF
	}
	n, err := p.ReadCloser.Read(b)
	if n > 0 {
		p.body = append(p.body, b[:n]...)
	}
	return n, err
}

func (p *interceptor) Close() error {
	if p.complete != nil {
		p.complete(p.body)
	}
	if p.ReadCloser == nil {
		return nil
	}
	return p.ReadCloser.Close()
}
