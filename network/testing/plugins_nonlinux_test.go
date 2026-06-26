//go:build windows

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

package testing

import (
	"testing"

	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/network"

	"github.com/stretchr/testify/assert"
)

func TestInitNonLinux(t *testing.T) {
	plug := &network.NoopNetworkPlugin{}
	// On non-Linux, Init should return immediately without touching sysctl.
	// If the sysctl path were reached, it would panic because Sysctl is nil.
	err := plug.Init(NewFakeHost(nil), config.HairpinNone, "10.0.0.0/8", network.UseDefaultMTU)
	assert.NoError(t, err)
}
