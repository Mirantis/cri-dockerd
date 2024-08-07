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
	"io"
	"time"

	dockertypes "github.com/docker/docker/api/types"
	"k8s.io/client-go/tools/remotecommand"

	dockercontainer "github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"

	"github.com/Mirantis/cri-dockerd/libdocker"

	"k8s.io/apimachinery/pkg/util/runtime"
)

type dockerExitError struct {
	Inspect *dockercontainer.ExecInspect
}

func (d *dockerExitError) String() string {
	return d.Error()
}

func (d *dockerExitError) Error() string {
	return fmt.Sprintf("Error executing in Docker Container: %d", d.Inspect.ExitCode)
}

func (d *dockerExitError) Exited() bool {
	return !d.Inspect.Running
}

func (d *dockerExitError) ExitStatus() int {
	return d.Inspect.ExitCode
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

// NativeExecHandler executes commands in Docker containers using Docker's exec API.
type NativeExecHandler struct{}

// ExecInContainer executes the cmd in container using the Docker's exec API
func (*NativeExecHandler) ExecInContainer(
	ctx context.Context,
	client libdocker.DockerClientInterface,
	container *dockertypes.ContainerJSON,
	cmd []string,
	stdin io.Reader,
	stdout, stderr io.WriteCloser,
	tty bool,
	resize <-chan remotecommand.TerminalSize,
	timeout time.Duration,
) error {
	done := make(chan struct{})
	defer close(done)

	createOpts := dockercontainer.ExecOptions{
		Cmd:          cmd,
		AttachStdin:  stdin != nil,
		AttachStdout: stdout != nil,
		AttachStderr: stderr != nil,
		Tty:          tty,
	}
	execObj, err := client.CreateExec(container.ID, createOpts)
	if err != nil {
		return fmt.Errorf("failed to exec in container - Exec setup failed - %v", err)
	}

	// Have to start this before the call to client.StartExec because client.StartExec is a blocking
	// call :-( Otherwise, resize events don't get processed and the terminal never resizes.
	//
	// We also have to delay attempting to send a terminal resize request to docker until after the
	// exec has started; otherwise, the initial resize request will fail.
	execStarted := make(chan struct{})
	go func() {
		select {
		case <-execStarted:
			// client.StartExec has started the exec, so we can start resizing
		case <-done:
			// ExecInContainer has returned, so short-circuit
			return
		}

		handleResizing(resize, func(size remotecommand.TerminalSize) {
			client.ResizeExecTTY(execObj.ID, uint(size.Height), uint(size.Width))
		})
	}()

	startOpts := dockercontainer.ExecStartOptions{Detach: false, Tty: tty}
	streamOpts := libdocker.StreamOptions{
		InputStream:  stdin,
		OutputStream: stdout,
		ErrorStream:  stderr,
		RawTerminal:  tty,
		ExecStarted:  execStarted,
	}

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// StartExec is a blocking call, so we need to run it concurrently and catch
	// its error in a channel
	execErr := make(chan error, 1)
	go func() {
		execErr <- client.StartExec(execObj.ID, startOpts, streamOpts)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-execErr:
		if err != nil {
			return err
		}
	}

	// InspectExec may not always return latest state of exec, so call it a few times until
	// it returns an exec inspect that shows that the process is no longer running.
	retries := 0
	maxRetries := 5
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		inspect, err := client.InspectExec(execObj.ID)
		if err != nil {
			return err
		}

		if !inspect.Running {
			if inspect.ExitCode != 0 {
				return &dockerExitError{inspect}
			}

			return nil
		}

		retries++
		if retries == maxRetries {
			logrus.Errorf(
				"Exec session in the container terminated but process still running! Session %s | Container %s",
				execObj.ID,
				container.ID,
			)
			return nil
		}

		<-ticker.C
	}
}
