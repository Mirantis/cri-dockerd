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
	"errors"
	"fmt"
	"github.com/Mirantis/cri-dockerd/config"
	"io"
	"regexp"
	"strings"
	"sync/atomic"

	dockerfilters "github.com/docker/docker/api/types/filters"
)

const (
	annotationPrefix     = "annotation."
	securityOptSeparator = '='
)

var (
	conflictRE = regexp.MustCompile(
		`Conflict. (?:.)+ is already in use by container \"?([0-9a-z]+)\"?`,
	)

	// this is hacky, but extremely common.
	// if a container starts but the executable file is not found, runc gives a message that matches
	startRE = regexp.MustCompile(`\\\\\\\"(.*)\\\\\\\": executable file not found`)
	defaultSeccompOpt = []DockerOpt{{"seccomp", config.SeccompProfileNameUnconfined, ""}}

	errMaximumWrite = errors.New("maximum write")
)

// makeLabels converts annotations to labels and merge them with the given
// labels. This is necessary because docker does not support annotations;
// we *fake* annotations using labels. Note that docker labels are not
// updatable.
func makeLabels(labels, annotations map[string]string) map[string]string {
	merged := make(map[string]string)
	for k, v := range labels {
		merged[k] = v
	}
	for k, v := range annotations {
		// Assume there won't be conflict.
		merged[fmt.Sprintf("%s%s", annotationPrefix, k)] = v
	}
	return merged
}

// extractLabels converts raw docker labels to the CRI labels and annotations.
// It also filters out internal labels used by this shim.
func extractLabels(input map[string]string) (map[string]string, map[string]string) {
	labels := make(map[string]string)
	annotations := make(map[string]string)
	for k, v := range input {
		// Check if the key is used internally by the shim.
		internal := false
		for _, internalKey := range internalLabelKeys {
			if k == internalKey {
				internal = true
				break
			}
		}
		if internal {
			continue
		}

		// Delete the container name label for the sandbox. It is added in the shim,
		// should not be exposed via CRI.
		if k == config.KubernetesContainerNameLabel &&
			input[containerTypeLabelKey] == containerTypeLabelSandbox {
			continue
		}

		// Check if the label should be treated as an annotation.
		if strings.HasPrefix(k, annotationPrefix) {
			annotations[strings.TrimPrefix(k, annotationPrefix)] = v
			continue
		}
		labels[k] = v
	}
	return labels, annotations
}

// dockerFilter wraps around dockerfilters.Args and provides methods to modify
// the filter easily.
type dockerFilter struct {
	args *dockerfilters.Args
}

func NewDockerFilter(args *dockerfilters.Args) *dockerFilter {
	return &dockerFilter{args: args}
}

func (f *dockerFilter) Add(key, value string) {
	f.args.Add(key, value)
}

func (f *dockerFilter) AddLabel(key, value string) {
	f.Add("label", fmt.Sprintf("%s=%s", key, value))
}

// sharedWriteLimiter limits the total output written across one or more streams.
type SharedWriteLimiter struct {
	delegate io.Writer
	limit    *int64
}

func (w SharedWriteLimiter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	limit := atomic.LoadInt64(w.limit)
	if limit <= 0 {
		return 0, errMaximumWrite
	}
	var truncated bool
	if limit < int64(len(p)) {
		p = p[0:limit]
		truncated = true
	}
	n, err := w.delegate.Write(p)
	if n > 0 {
		atomic.AddInt64(w.limit, -1*int64(n))
	}
	if err == nil && truncated {
		err = errMaximumWrite
	}
	return n, err
}

func SharedLimitWriter(w io.Writer, limit *int64) io.Writer {
	if w == nil {
		return nil
	}
	return &SharedWriteLimiter{
		delegate: w,
		limit:    limit,
	}
}

// fmtDockerOpts formats the docker security options using the given separator.
func FmtDockerOpts(opts []DockerOpt, sep rune) []string {
	fmtOpts := make([]string, len(opts))
	for i, opt := range opts {
		fmtOpts[i] = fmt.Sprintf("%s%c%s", opt.key, sep, opt.value)
	}
	return fmtOpts
}

type DockerOpt struct {
	// The key-value pair passed to docker.
	key, value string
	// The alternative value to use in log/event messages.
	msg string
}

// GetKV exposes key/value from  dockerOpt.
func (d DockerOpt) GetKV() (string, string) {
	return d.key, d.value
}

