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

	"github.com/opencontainers/selinux/go-selinux/label"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// selinuxLabelUser returns the fragment of a Docker security opt that
// describes the SELinux user. Note that strictly speaking this is not
// actually the name of the security opt, but a fragment of the whole key-
// value pair necessary to set the opt.
func selinuxLabelUser(separator rune) string {
	return fmt.Sprintf("label%cuser", separator)
}

// selinuxLabelRole returns the fragment of a Docker security opt that
// describes the SELinux role. Note that strictly speaking this is not
// actually the name of the security opt, but a fragment of the whole key-
// value pair necessary to set the opt.
func selinuxLabelRole(separator rune) string {
	return fmt.Sprintf("label%crole", separator)
}

// selinuxLabelType returns the fragment of a Docker security opt that
// describes the SELinux type. Note that strictly speaking this is not
// actually the name of the security opt, but a fragment of the whole key-
// value pair necessary to set the opt.
func selinuxLabelType(separator rune) string {
	return fmt.Sprintf("label%ctype", separator)
}

// selinuxLabelLevel returns the fragment of a Docker security opt that
// describes the SELinux level. Note that strictly speaking this is not
// actually the name of the security opt, but a fragment of the whole key-
// value pair necessary to set the opt.
func selinuxLabelLevel(separator rune) string {
	return fmt.Sprintf("label%clevel", separator)
}

// addSELinuxOptions adds SELinux options to config using the given
// separator.
func addSELinuxOptions(
	config []string,
	selinuxOpts *runtimeapi.SELinuxOption,
	separator rune,
) []string {
	// Note, strictly speaking, we are actually mutating the values of these
	// keys, rather than formatting name and value into a string.  Docker re-
	// uses the same option name multiple times (it's just 'label') with
	// different values which are themselves key-value pairs.  For example,
	// the SELinux type is represented by the security opt:
	//
	//   label<separator>type:<selinux_type>
	//
	// In Docker API versions before 1.23, the separator was the `:` rune; in
	// API version 1.23 it changed to the `=` rune.
	config = modifySecurityOption(config, selinuxLabelUser(separator), selinuxOpts.User)
	config = modifySecurityOption(config, selinuxLabelRole(separator), selinuxOpts.Role)
	config = modifySecurityOption(config, selinuxLabelType(separator), selinuxOpts.Type)
	config = modifySecurityOption(config, selinuxLabelLevel(separator), selinuxOpts.Level)

	return config
}

// modifySecurityOption adds the security option of name to the config array
// with value in the form of name:value.
func modifySecurityOption(config []string, name, value string) []string {
	if len(value) > 0 {
		config = append(config, fmt.Sprintf("%s:%s", name, value))
	}

	return config
}

func selinuxMountLabel(selinuxOpts *runtimeapi.SELinuxOption) (string, error) {
	var opts []string
	if selinuxOpts != nil {
		if selinuxOpts.User != "" {
			opts = append(opts, "user:"+selinuxOpts.User)
		}
		if selinuxOpts.Role != "" {
			opts = append(opts, "role:"+selinuxOpts.Role)
		}
		if selinuxOpts.Type != "" {
			opts = append(opts, "type:"+selinuxOpts.Type)
		}
		if selinuxOpts.Level != "" {
			opts = append(opts, "level:"+selinuxOpts.Level)
		}
	}
	if len(opts) == 0 {
		opts = []string{"disable"}
	}
	_, s, err := label.InitLabels(opts)
	return s, err
}
