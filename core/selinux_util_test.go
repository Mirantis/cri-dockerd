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
	"reflect"
	"testing"

	"github.com/opencontainers/selinux/go-selinux"
	"github.com/stretchr/testify/assert"
)

func TestModifySecurityOptions(t *testing.T) {
	testCases := []struct {
		name     string
		config   []string
		optName  string
		optVal   string
		expected []string
	}{
		{
			name:     "Empty val",
			config:   []string{"a:b", "c:d"},
			optName:  "optA",
			optVal:   "",
			expected: []string{"a:b", "c:d"},
		},
		{
			name:     "Valid",
			config:   []string{"a:b", "c:d"},
			optName:  "e",
			optVal:   "f",
			expected: []string{"a:b", "c:d", "e:f"},
		},
	}

	for _, tc := range testCases {
		actual := modifySecurityOption(tc.config, tc.optName, tc.optVal)
		if !reflect.DeepEqual(tc.expected, actual) {
			t.Errorf(
				"Failed to apply options correctly for tc: %s.  Expected: %v but got %v",
				tc.name,
				tc.expected,
				actual,
			)
		}
	}
}

func TestSELinuxMountLabel(t *testing.T) {
	selinuxOpts := inputSELinuxOptions()
	s, err := selinuxMountLabel(selinuxOpts)
	assert.NoError(t, err)
	t.Logf("mount label for %+v=%q", selinuxOpts, s)
	if selinux.GetEnabled() {
		assert.NotEmpty(t, s)
	} else {
		assert.Empty(t, s)
	}
}
