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

package streaming

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/remotecommand"

	dockertypes "github.com/docker/docker/api/types"
	dockercontainer "github.com/docker/docker/api/types/container"

	"k8s.io/kubelet/pkg/cri/streaming"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

type StreamingRuntime struct {
	Client      libdocker.DockerClientInterface
	ExecHandler ExecHandler
}

// ExecHandler knows how to execute a command in a running Docker container.
type ExecHandler interface {
	ExecInContainer(
		ctx context.Context,
		client libdocker.DockerClientInterface,
		container *dockertypes.ContainerJSON,
		cmd []string,
		stdin io.Reader,
		stdout, stderr io.WriteCloser,
		tty bool,
		resize <-chan remotecommand.TerminalSize,
		timeout time.Duration,
	) error
}

var _ streaming.Runtime = &StreamingRuntime{}

func (r *StreamingRuntime) Exec(
	ctx context.Context,
	containerID string,
	cmd []string,
	in io.Reader,
	out, err io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
) error {
	return r.ExecWithContext(context.TODO(), containerID, cmd, in, out, err, tty, resize, 0)
}

// ExecWithContext adds a context.
func (r *StreamingRuntime) ExecWithContext(
	ctx context.Context,
	containerID string,
	cmd []string,
	in io.Reader,
	out, errw io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
	timeout time.Duration,
) error {
	container, err := libdocker.CheckContainerStatus(r.Client, containerID)
	if err != nil {
		return err
	}

	return r.ExecHandler.ExecInContainer(
		ctx,
		r.Client,
		container,
		cmd,
		in,
		out,
		errw,
		tty,
		resize,
		timeout,
	)
}

func (r *StreamingRuntime) Attach(
	ctx context.Context,
	containerID string,
	in io.Reader,
	out, errw io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
) error {
	_, err := libdocker.CheckContainerStatus(r.Client, containerID)
	if err != nil {
		return err
	}

	return attachContainer(r.Client, containerID, in, out, errw, tty, resize)
}

func (r *StreamingRuntime) PortForward(
	ctx context.Context,
	podSandboxID string,
	port int32,
	stream io.ReadWriteCloser,
) error {
	if port < 0 || port > math.MaxUint16 {
		return fmt.Errorf("invalid port %d", port)
	}
	return r.portForward(podSandboxID, port, stream)
}

func attachContainer(
	client libdocker.DockerClientInterface,
	containerID string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
) error {
	// Have to start this before the call to client.AttachToContainer because client.AttachToContainer is a blocking
	// call :-( Otherwise, resize events don't get processed and the terminal never resizes.
	handleResizing(resize, func(size remotecommand.TerminalSize) {
		client.ResizeContainerTTY(containerID, uint(size.Height), uint(size.Width))
	})

	opts := dockercontainer.AttachOptions{
		Stream: true,
		Stdin:  stdin != nil,
		Stdout: stdout != nil,
		Stderr: stderr != nil,
	}
	sopts := libdocker.StreamOptions{
		InputStream:  stdin,
		OutputStream: stdout,
		ErrorStream:  stderr,
		RawTerminal:  tty,
	}
	return client.AttachToContainer(containerID, opts, sopts)
}

func handleResizing(resize <-chan remotecommand.TerminalSize, resizeFunc func(size remotecommand.TerminalSize)) {
	if resize == nil {
		return
	}

	go func() {
		defer runtime.HandleCrash()

		for size := range resize {
			if size.Height < 1 || size.Width < 1 {
				continue
			}
			resizeFunc(size)
		}
	}()
}
