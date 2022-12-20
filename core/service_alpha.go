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
	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/utils"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	runtimeapi_alpha "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

func (as *dockerServiceAlpha) RunPodSandbox(ctx context.Context, r *runtimeapi_alpha.RunPodSandboxRequest) (res *runtimeapi_alpha.RunPodSandboxResponse, err error) {
	var v1Request v1.RunPodSandboxRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.RunPodSandboxResponse
	v1Response, err = as.ds.RunPodSandbox(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.RunPodSandboxResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// ListPodSandbox returns a list of Sandbox.
func (as *dockerServiceAlpha) ListPodSandbox(
	ctx context.Context,
	r *runtimeapi_alpha.ListPodSandboxRequest,
) (res *runtimeapi_alpha.ListPodSandboxResponse, err error) {
	// Convert docker containers to runtime api sandboxes.
	var v1Request v1.ListPodSandboxRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}

	var v1Response *v1.ListPodSandboxResponse
	v1Response, err = as.ds.ListPodSandbox(context.Background(), &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ListPodSandboxResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
	}
	return res, err
}

func (as *dockerServiceAlpha) PodSandboxStatus(
	ctx context.Context,
	r *runtimeapi_alpha.PodSandboxStatusRequest,
) (res *runtimeapi_alpha.PodSandboxStatusResponse, err error) {
	var v1Request v1.PodSandboxStatusRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.PodSandboxStatusResponse
	v1Response, err = as.ds.PodSandboxStatus(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.PodSandboxStatusResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// StopPodSandbox returns a list of Sandbox.
func (as *dockerServiceAlpha) StopPodSandbox(
	ctx context.Context,
	r *runtimeapi_alpha.StopPodSandboxRequest,
) (res *runtimeapi_alpha.StopPodSandboxResponse, err error) {
	var v1Request v1.StopPodSandboxRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.StopPodSandboxResponse
	v1Response, err = as.ds.StopPodSandbox(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.StopPodSandboxResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// RemovePodSandbox returns a list of Sandbox.
func (as *dockerServiceAlpha) RemovePodSandbox(
	ctx context.Context,
	r *runtimeapi_alpha.RemovePodSandboxRequest,
) (res *runtimeapi_alpha.RemovePodSandboxResponse, err error) {
	var v1Request v1.RemovePodSandboxRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.RemovePodSandboxResponse
	v1Response, err = as.ds.RemovePodSandbox(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.RemovePodSandboxResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// PortForward returns a list of Sandbox.
func (as *dockerServiceAlpha) PortForward(
	ctx context.Context,
	r *runtimeapi_alpha.PortForwardRequest,
) (res *runtimeapi_alpha.PortForwardResponse, err error) {
	var v1Request v1.PortForwardRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.PortForwardResponse
	v1Response, err = as.ds.PortForward(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.PortForwardResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// CreateContainer returns a list of Sandbox.
func (as *dockerServiceAlpha) CreateContainer(
	ctx context.Context,
	r *runtimeapi_alpha.CreateContainerRequest,
) (res *runtimeapi_alpha.CreateContainerResponse, err error) {
	var v1Request v1.CreateContainerRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.CreateContainerResponse
	v1Response, err = as.ds.CreateContainer(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.CreateContainerResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// StartContainer returns a list of Sandbox.
func (as *dockerServiceAlpha) StartContainer(
	ctx context.Context,
	r *runtimeapi_alpha.StartContainerRequest,
) (res *runtimeapi_alpha.StartContainerResponse, err error) {
	var v1Request v1.StartContainerRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.StartContainerResponse
	v1Response, err = as.ds.StartContainer(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.StartContainerResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// ListContainers returns a list of Sandbox.
func (as *dockerServiceAlpha) ListContainers(
	ctx context.Context,
	r *runtimeapi_alpha.ListContainersRequest,
) (res *runtimeapi_alpha.ListContainersResponse, err error) {
	var v1Request v1.ListContainersRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ListContainersResponse
	v1Response, err = as.ds.ListContainers(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ListContainersResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// ContainerStatus returns a list of Sandbox.
func (as *dockerServiceAlpha) ContainerStatus(
	ctx context.Context,
	r *runtimeapi_alpha.ContainerStatusRequest,
) (res *runtimeapi_alpha.ContainerStatusResponse, err error) {
	var v1Request v1.ContainerStatusRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ContainerStatusResponse
	v1Response, err = as.ds.ContainerStatus(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ContainerStatusResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// StopContainer returns a list of Sandbox.
func (as *dockerServiceAlpha) StopContainer(
	ctx context.Context,
	r *runtimeapi_alpha.StopContainerRequest,
) (res *runtimeapi_alpha.StopContainerResponse, err error) {
	var v1Request v1.StopContainerRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.StopContainerResponse
	v1Response, err = as.ds.StopContainer(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.StopContainerResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// RemoveContainer returns a list of Sandbox.
func (as *dockerServiceAlpha) RemoveContainer(
	ctx context.Context,
	r *runtimeapi_alpha.RemoveContainerRequest,
) (res *runtimeapi_alpha.RemoveContainerResponse, err error) {
	var v1Request v1.RemoveContainerRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.RemoveContainerResponse
	v1Response, err = as.ds.RemoveContainer(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.RemoveContainerResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// ExecSync returns a list of Sandbox.
func (as *dockerServiceAlpha) ExecSync(
	ctx context.Context,
	r *runtimeapi_alpha.ExecSyncRequest,
) (res *runtimeapi_alpha.ExecSyncResponse, err error) {
	var v1Request v1.ExecSyncRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ExecSyncResponse
	v1Response, err = as.ds.ExecSync(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ExecSyncResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) Exec(
	ctx context.Context,
	r *runtimeapi_alpha.ExecRequest,
) (res *runtimeapi_alpha.ExecResponse, err error) {
	var v1Request v1.ExecRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ExecResponse
	v1Response, err = as.ds.Exec(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ExecResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) Attach(
	ctx context.Context,
	r *runtimeapi_alpha.AttachRequest,
) (res *runtimeapi_alpha.AttachResponse, err error) {
	var v1Request v1.AttachRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.AttachResponse
	v1Response, err = as.ds.Attach(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.AttachResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) UpdateContainerResources(
	ctx context.Context,
	r *runtimeapi_alpha.UpdateContainerResourcesRequest,
) (res *runtimeapi_alpha.UpdateContainerResourcesResponse, err error) {
	var v1Request v1.UpdateContainerResourcesRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.UpdateContainerResourcesResponse
	v1Response, err = as.ds.UpdateContainerResources(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.UpdateContainerResourcesResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) PullImage(
	ctx context.Context,
	r *runtimeapi_alpha.PullImageRequest,
) (res *runtimeapi_alpha.PullImageResponse, err error) {
	var v1Request v1.PullImageRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.PullImageResponse
	v1Response, err = as.ds.PullImage(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.PullImageResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ListImages(
	ctx context.Context,
	r *runtimeapi_alpha.ListImagesRequest,
) (res *runtimeapi_alpha.ListImagesResponse, err error) {
	var v1Request v1.ListImagesRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ListImagesResponse
	v1Response, err = as.ds.ListImages(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ListImagesResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ImageStatus(
	ctx context.Context,
	r *runtimeapi_alpha.ImageStatusRequest,
) (res *runtimeapi_alpha.ImageStatusResponse, err error) {
	var v1Request v1.ImageStatusRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ImageStatusResponse
	v1Response, err = as.ds.ImageStatus(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ImageStatusResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) RemoveImage(
	ctx context.Context,
	r *runtimeapi_alpha.RemoveImageRequest,
) (res *runtimeapi_alpha.RemoveImageResponse, err error) {
	var v1Request v1.RemoveImageRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.RemoveImageResponse
	v1Response, err = as.ds.RemoveImage(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.RemoveImageResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ImageFsInfo(
	ctx context.Context,
	r *runtimeapi_alpha.ImageFsInfoRequest,
) (res *runtimeapi_alpha.ImageFsInfoResponse, err error) {
	var v1Request v1.ImageFsInfoRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ImageFsInfoResponse
	v1Response, err = as.ds.ImageFsInfo(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ImageFsInfoResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ContainerStats(
	ctx context.Context,
	r *runtimeapi_alpha.ContainerStatsRequest,
) (res *runtimeapi_alpha.ContainerStatsResponse, err error) {
	var v1Request v1.ContainerStatsRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ContainerStatsResponse
	v1Response, err = as.ds.ContainerStats(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ContainerStatsResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ListContainerStats(
	ctx context.Context,
	r *runtimeapi_alpha.ListContainerStatsRequest,
) (res *runtimeapi_alpha.ListContainerStatsResponse, err error) {
	var v1Request v1.ListContainerStatsRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ListContainerStatsResponse
	v1Response, err = as.ds.ListContainerStats(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ListContainerStatsResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) Status(
	ctx context.Context,
	r *runtimeapi_alpha.StatusRequest,
) (res *runtimeapi_alpha.StatusResponse, err error) {
	var v1Request v1.StatusRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.StatusResponse
	v1Response, err = as.ds.Status(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.StatusResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) UpdateRuntimeConfig(
	ctx context.Context,
	r *runtimeapi_alpha.UpdateRuntimeConfigRequest,
) (res *runtimeapi_alpha.UpdateRuntimeConfigResponse, err error) {
	var v1Request v1.UpdateRuntimeConfigRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.UpdateRuntimeConfigResponse
	v1Response, err = as.ds.UpdateRuntimeConfig(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.UpdateRuntimeConfigResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

func (as *dockerServiceAlpha) ReopenContainerLog(
	ctx context.Context,
	r *runtimeapi_alpha.ReopenContainerLogRequest,
) (res *runtimeapi_alpha.ReopenContainerLogResponse, err error) {
	var v1Request v1.ReopenContainerLogRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.ReopenContainerLogResponse
	v1Response, err = as.ds.ReopenContainerLog(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.ReopenContainerLogResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		return res, err
	}
	return nil, err
}

// Version returns the runtime name, runtime version and runtime API version
func (as *dockerServiceAlpha) Version(
	ctx context.Context,
	r *runtimeapi_alpha.VersionRequest,
) (res *runtimeapi_alpha.VersionResponse, err error) {
	var v1Request v1.VersionRequest
	if err := utils.AlphaReqToV1Req(r, &v1Request); err != nil {
		return nil, err
	}
	var v1Response *v1.VersionResponse
	v1Response, err = as.ds.Version(ctx, &v1Request)
	if v1Response != nil {
		resp := &runtimeapi_alpha.VersionResponse{}
		err = utils.V1ResponseToAlphaResponse(v1Response, resp)
		if err == nil {
			res = resp
		}
		res.RuntimeApiVersion = config.CRIVersionAlpha
		return res, err
	}
	return nil, err
}
