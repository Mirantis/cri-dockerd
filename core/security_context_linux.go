// build +linux

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

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

func (ds *dockerService) getSecurityOpts(seccomp *runtimeapi.SecurityProfile, privileged bool, separator rune) ([]string, error) {
	// Apply seccomp options.
	seccompSecurityOpts, err := getSeccompSecurityOpts(seccomp, privileged, separator)
	if err != nil {
		return nil, fmt.Errorf("failed to generate seccomp security options for container: %v", err)
	}

	return seccompSecurityOpts, nil
}

func (ds *dockerService) getSandBoxSecurityOpts(separator rune) []string {
	// run sandbox with no-new-privileges and using runtime/default
	// sending no "seccomp=" means docker will use default profile
	return []string{"no-new-privileges"}
}
