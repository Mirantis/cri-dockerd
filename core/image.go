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
	"net/http"
	"strconv"
	"strings"

	dockerfilters "github.com/docker/docker/api/types/filters"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/pkg/jsonmessage"

	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

// This file implements methods in ImageManagerService.

func (ds *dockerService) sandboxImage() string {
	image := defaultSandboxImage
	podSandboxImage := ds.podSandboxImage
	if len(podSandboxImage) != 0 {
		image = podSandboxImage
	}
	return image
}

func isPinned(image string, tags []string) bool {
	pinned := false
	for _, tag := range tags {
		if tag == image {
			pinned = true
			break
		}
	}
	return pinned
}

// ListImages lists existing images.
func (ds *dockerService) ListImages(
	_ context.Context,
	r *runtimeapi.ListImagesRequest,
) (*runtimeapi.ListImagesResponse, error) {
	filter := r.GetFilter()
	opts := dockerimage.ListOptions{}
	if filter != nil {
		if filter.GetImage().GetImage() != "" {
			opts.Filters = dockerfilters.NewArgs()
			opts.Filters.Add("reference", filter.GetImage().GetImage())
		}
	}

	images, err := ds.client.ListImages(opts)
	if err != nil {
		return nil, err
	}
	image := ds.sandboxImage()

	result := make([]*runtimeapi.Image, 0, len(images))
	for _, i := range images {
		pinned := isPinned(image, i.RepoTags)
		apiImage, err := imageToRuntimeAPIImage(&i, pinned)
		if err != nil {
			logrus.Infof("Failed to convert docker API image %v to runtime API image: %v", i, err)
			continue
		}
		result = append(result, apiImage)
	}
	return &runtimeapi.ListImagesResponse{Images: result}, nil
}

// ImageStatus returns the status of the image, returns nil if the image doesn't present.
func (ds *dockerService) ImageStatus(
	_ context.Context,
	r *runtimeapi.ImageStatusRequest,
) (*runtimeapi.ImageStatusResponse, error) {
	image := r.GetImage()

	imageInspect, err := ds.client.InspectImageByRef(image.Image)
	if err != nil {
		if !libdocker.IsImageNotFoundError(err) {
			return nil, err
		}
		imageInspect, err = ds.client.InspectImageByID(image.Image)
		if err != nil {
			if libdocker.IsImageNotFoundError(err) {
				return &runtimeapi.ImageStatusResponse{}, nil
			}
			return nil, err
		}
	}

	pinned := isPinned(ds.sandboxImage(), imageInspect.RepoTags)
	imageStatus, err := imageInspectToRuntimeAPIImage(imageInspect, pinned)
	if err != nil {
		return nil, err
	}

	res := runtimeapi.ImageStatusResponse{Image: imageStatus}
	if r.GetVerbose() {
		imageHistory, err := ds.client.ImageHistory(imageInspect.ID)
		if err != nil {
			return nil, err
		}
		imageInfo, err := imageInspectToRuntimeAPIImageInfo(imageInspect, imageHistory)
		if err != nil {
			return nil, err
		}
		res.Info = imageInfo
	}
	return &res, nil
}

// PullImage pulls an image with authentication config.
func (ds *dockerService) PullImage(
	_ context.Context,
	r *runtimeapi.PullImageRequest,
) (*runtimeapi.PullImageResponse, error) {
	image := r.GetImage()
	auth := r.GetAuth()
	authConfig := dockerregistry.AuthConfig{}

	if auth != nil {
		authConfig.Username = auth.Username
		authConfig.Password = auth.Password
		authConfig.ServerAddress = auth.ServerAddress
		authConfig.IdentityToken = auth.IdentityToken
		authConfig.RegistryToken = auth.RegistryToken
	}
	err := ds.client.PullImage(image.Image,
		authConfig,
		dockerimage.PullOptions{},
	)
	if err != nil {
		return nil, filterHTTPError(err, image.Image)
	}

	imageRef, err := getImageRef(ds.client, image.Image)
	if err != nil {
		return nil, err
	}

	return &runtimeapi.PullImageResponse{ImageRef: imageRef}, nil
}

// RemoveImage removes the image.
func (ds *dockerService) RemoveImage(
	_ context.Context,
	r *runtimeapi.RemoveImageRequest,
) (*runtimeapi.RemoveImageResponse, error) {
	image := r.GetImage()
	// If the image has multiple tags, we need to remove all the tags
	// of kubelet, but we should still clarify this in CRI.
	imageInspect, err := ds.client.InspectImageByID(image.Image)

	// dockerclient.InspectImageByID doesn't work with digest and repoTags,
	// it is safe to continue removing it since there is another check below.
	if err != nil && !libdocker.IsImageNotFoundError(err) {
		return nil, err
	}

	if imageInspect == nil {
		// image is nil, assuming it doesn't exist.
		return &runtimeapi.RemoveImageResponse{}, nil
	}

	// An image can have different numbers of RepoTags and RepoDigests.
	// Iterating over both of them plus the image ID ensures the image really got removed.
	// It also prevents images from being deleted, which actually are deletable using this approach.
	var images []string
	images = append(images, imageInspect.RepoTags...)
	images = append(images, imageInspect.RepoDigests...)
	images = append(images, image.Image)

	for _, image := range images {
		if _, err := ds.client.RemoveImage(image, dockerimage.RemoveOptions{PruneChildren: true}); err != nil &&
			!libdocker.IsImageNotFoundError(err) {
			return nil, err
		}
	}

	return &runtimeapi.RemoveImageResponse{}, nil
}

// getImageRef returns the image digest if exists, or else returns the image ID.
func getImageRef(client libdocker.DockerClientInterface, image string) (string, error) {
	img, err := client.InspectImageByRef(image)
	if err != nil {
		return "", err
	}
	if img == nil {
		return "", fmt.Errorf("unable to inspect image %s", image)
	}

	// Returns the digest if it exist.
	if len(img.RepoDigests) > 0 {
		return img.RepoDigests[0], nil
	}

	return img.ID, nil
}

func filterHTTPError(err error, image string) error {
	// docker/docker/pull/11314 prints detailed error info for docker pull.
	// When it hits 502, it returns a verbose html output including an inline svg,
	// which makes the output of kubectl get pods much harder to parse.
	// Here converts such verbose output to a concise one.
	jerr, ok := err.(*jsonmessage.JSONError)
	if ok && (jerr.Code == http.StatusBadGateway ||
		jerr.Code == http.StatusServiceUnavailable ||
		jerr.Code == http.StatusGatewayTimeout) {
		return fmt.Errorf("RegistryUnavailable: %v", err)
	}
	return err

}

// parseUserFromImageUser splits the user out of an user:group string.
func parseUserFromImageUser(id string) string {
	if id == "" {
		return id
	}
	// split instances where the id may contain user:group
	if strings.Contains(id, ":") {
		return strings.Split(id, ":")[0]
	}
	// no group, just return the id
	return id
}

// getUserFromImageUser gets uid or user name of the image user.
// If user is numeric, it will be treated as uid; or else, it is treated as user name.
func getUserFromImageUser(imageUser string) (*int64, string) {
	user := parseUserFromImageUser(imageUser)
	// return both nil if user is not specified in the image.
	if user == "" {
		return nil, ""
	}
	// user could be either uid or user name. Try to interpret as numeric uid.
	uid, err := strconv.ParseInt(user, 10, 64)
	if err != nil {
		// If user is non numeric, assume it's user name.
		return nil, user
	}
	// If user is a numeric uid.
	return &uid, ""
}
