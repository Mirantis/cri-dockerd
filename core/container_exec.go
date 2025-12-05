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
	"bytes"
	"context"
	"errors"
	"time"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/streaming"
	"github.com/Mirantis/cri-dockerd/utils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// ExecSync executes a command in the container, and returns the stdout output.
// If command exits with a non-zero exit code, an error is returned.
func (ds *dockerService) ExecSync(
	ctx context.Context,
	req *v1.ExecSyncRequest,
) (*v1.ExecSyncResponse, error) {
	timeout := time.Duration(req.Timeout) * time.Second
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err := ds.streamingRuntime.ExecWithContext(ctx, req.ContainerId, req.Cmd,
		nil, // in
		utils.WriteCloserWrapper(utils.LimitWriter(&stdoutBuffer, maxMsgSize)),
		utils.WriteCloserWrapper(utils.LimitWriter(&stderrBuffer, maxMsgSize)),
		false, // tty
		nil,   // resize
		timeout)

	// kubelet's backend runtime expects a grpc error with status code DeadlineExceeded on time out.
	if errors.Is(err, context.DeadlineExceeded) {
		return nil, status.Error(codes.DeadlineExceeded, err.Error())
	}

	var exitCode int32
	if err != nil {
		exitError, ok := err.(utils.ExitError)
		if !ok {
			return nil, err
		}

		exitCode = int32(exitError.ExitStatus())
	}
	return &v1.ExecSyncResponse{
		Stdout:   stdoutBuffer.Bytes(),
		Stderr:   stderrBuffer.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// Exec prepares a streaming endpoint to execute a command in the container, and returns the address.
func (ds *dockerService) Exec(
	_ context.Context,
	req *v1.ExecRequest,
) (*v1.ExecResponse, error) {
	if ds.streamingServer == nil {
		return nil, streaming.NewErrorStreamingDisabled("exec")
	}
	_, err := libdocker.CheckContainerStatus(ds.client, req.ContainerId)
	if err != nil {
		return nil, err
	}
	return ds.streamingServer.GetExec(req)
}
