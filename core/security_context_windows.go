//go:build windows
// +build windows

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
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func (ds *dockerService) getSecurityOpts(
	seccompProfile *v1.SecurityProfile,
	privileged bool,
	separator rune,
) ([]string, error) {
	if seccompProfile != nil {
		logrus.Info("seccomp annotations are not supported on windows")
	}
	return nil, nil
}

func (ds *dockerService) getSandBoxSecurityOpts(separator rune) []string {
	// Currently, Windows container does not support privileged mode, so no no-new-privileges flag can be returned directly like Linux
	// If the future Windows container has new support for privileged mode, we can adjust it here
	return nil
}

// applyWindowsContainerSecurityContext updates docker container options according to security context.
func applyWindowsContainerSecurityContext(
	wsc *v1.WindowsContainerSecurityContext,
	config *container.Config,
	hc *container.HostConfig,
) {
	if wsc == nil {
		return
	}

	if wsc.GetRunAsUsername() != "" {
		config.User = wsc.GetRunAsUsername()
	}
}

// performPlatformSpecificContainerCleanup is responsible for doing any platform-specific cleanup
// after either the container creation has failed or the container has been removed.
func (ds *dockerService) performPlatformSpecificContainerCleanup(
	cleanupInfo *containerCleanupInfo,
) (errors []error) {
	if err := removeGMSARegistryValue(cleanupInfo); err != nil {
		errors = append(errors, err)
	}

	return
}

// platformSpecificContainerInitCleanup is called when cri-dockerd
// is starting, and is meant to clean up any cruft left by previous runs
// creating containers.
// Errors are simply logged, but don't prevent cri-dockerd from starting.
func (ds *dockerService) platformSpecificContainerInitCleanup() (errors []error) {
	return removeAllGMSARegistryValues()
}
