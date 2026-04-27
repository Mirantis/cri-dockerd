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
	"fmt"
	"reflect"
	"testing"

	dockertypes "github.com/docker/docker/api/types"
	dockerimage "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/Mirantis/cri-dockerd/libdocker"
)

func TestRemoveImage(t *testing.T) {
	tests := map[string]struct {
		image         dockertypes.ImageInspect
		calledDetails []libdocker.CalledDetail
	}{
		"single tag": {
			dockertypes.ImageInspect{ID: "1111", RepoTags: []string{"foo"}},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("inspect_image", nil),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"foo", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"1111", dockerimage.RemoveOptions{PruneChildren: true}},
				),
			},
		},
		"multiple tags": {
			dockertypes.ImageInspect{ID: "2222", RepoTags: []string{"foo", "bar"}},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("inspect_image", nil),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"foo", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"bar", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"2222", dockerimage.RemoveOptions{PruneChildren: true}},
				),
			},
		},
		"single tag multiple repo digests": {
			dockertypes.ImageInspect{
				ID:          "3333",
				RepoTags:    []string{"foo"},
				RepoDigests: []string{"foo@3333", "example.com/foo@3333"},
			},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("inspect_image", nil),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"foo", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"foo@3333", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{
						"example.com/foo@3333",
						dockerimage.RemoveOptions{PruneChildren: true},
					},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"3333", dockerimage.RemoveOptions{PruneChildren: true}},
				),
			},
		},
		"no tags multiple repo digests": {
			dockertypes.ImageInspect{
				ID:          "4444",
				RepoTags:    []string{},
				RepoDigests: []string{"foo@4444", "example.com/foo@4444"},
			},
			[]libdocker.CalledDetail{
				libdocker.NewCalledDetail("inspect_image", nil),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"foo@4444", dockerimage.RemoveOptions{PruneChildren: true}},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{
						"example.com/foo@4444",
						dockerimage.RemoveOptions{PruneChildren: true},
					},
				),
				libdocker.NewCalledDetail(
					"remove_image",
					[]interface{}{"4444", dockerimage.RemoveOptions{PruneChildren: true}},
				),
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ds, fakeDocker, _ := newTestDockerService()
			fakeDocker.InjectImageInspects([]dockertypes.ImageInspect{test.image})
			ds.RemoveImage(
				getTestCTX(),
				&runtimeapi.RemoveImageRequest{Image: &runtimeapi.ImageSpec{Image: test.image.ID}},
			)
			err := fakeDocker.AssertCallDetails(test.calledDetails...)
			assert.NoError(t, err)
		})
	}
}

func TestPullImage(t *testing.T) {
	ds, fakeDocker, _ := newTestDockerService()
	tests := map[string]struct {
		image         *runtimeapi.ImageSpec
		err           error
		expectedError string
		expectImage   bool
	}{
		"Json error": {
			&runtimeapi.ImageSpec{Image: "ubuntu"},
			&jsonmessage.JSONError{Code: 50, Message: "Json error"},
			"Json error",
			false,
		},
		"Bad gateway": {
			&runtimeapi.ImageSpec{Image: "ubuntu"},
			&jsonmessage.JSONError{
				Code:    502,
				Message: "<!doctype html>\n<html class=\"no-js\" lang=\"\">\n    <head>\n  </head>\n    <body>\n   <h1>Oops, there was an error!</h1>\n        <p>We have been contacted of this error, feel free to check out <a href=\"http://status.docker.com/\">status.docker.com</a>\n           to see if there is a bigger issue.</p>\n\n    </body>\n</html>",
			},
			"RegistryUnavailable",
			false,
		},
		"Successful Pull": {
			&runtimeapi.ImageSpec{Image: "ubuntu"},
			nil,
			"",
			true,
		},
	}
	for key, test := range tests {
		if test.err != nil {
			fakeDocker.InjectError("pull", test.err)
		}

		gotResp, gotErr := ds.PullImage(
			getTestCTX(),
			&runtimeapi.PullImageRequest{Image: test.image, Auth: &runtimeapi.AuthConfig{}},
		)
		if (len(test.expectedError) > 0) != (gotErr != nil) {
			t.Fatalf("expected err %q but got %v", test.expectedError, gotErr)
		}

		if len(test.expectedError) > 0 {
			require.Error(t, gotErr, fmt.Sprintf("TestCase [%s]", key))
			assert.Contains(t, gotErr.Error(), test.expectedError)
		}

		if test.expectImage {
			expectedResp := &runtimeapi.PullImageResponse{
				ImageRef: libdocker.FakePullImageIDMapping(test.image.Image),
			}

			if !reflect.DeepEqual(gotResp, expectedResp) {
				t.Errorf("expected pull response %v, got %v", expectedResp, gotResp)
			}
		}

	}
}
