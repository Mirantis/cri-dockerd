/*
Copyright 2026 Mirantis

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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPinnedImageMatcher(t *testing.T) {
	const versionLabel = "org.opencontainers.image.version"

	tests := map[string]struct {
		refs        []string
		labels      []string
		repoTags    []string
		repoDigests []string
		imageLabels map[string]string
		want        bool
	}{
		"no selectors pins nothing": {
			repoTags: []string{"nginx:1.27"},
			want:     false,
		},
		"exact reference": {
			refs:     []string{"nginx:1.27"},
			repoTags: []string{"nginx:1.27"},
			want:     true,
		},
		"exact reference does not match another tag": {
			refs:     []string{"nginx:1.27"},
			repoTags: []string{"nginx:1.28"},
			want:     false,
		},
		"bare repository pins every tag": {
			refs:     []string{"nginx"},
			repoTags: []string{"nginx:1.28"},
			want:     true,
		},
		"bare repository does not match a different repository": {
			refs:     []string{"nginx"},
			repoTags: []string{"redis:7.4"},
			want:     false,
		},
		"registry port is not mistaken for a tag": {
			refs:     []string{"localhost:5000/library/nginx"},
			repoTags: []string{"localhost:5000/library/nginx:1.27"},
			want:     true,
		},
		"prefix pattern": {
			refs:     []string{"library/ngin*"},
			repoTags: []string{"library/nginx:1.27"},
			want:     true,
		},
		"prefix pattern does not over-match": {
			refs:     []string{"library/ngin*"},
			repoTags: []string{"library/redis:7.4"},
			want:     false,
		},
		"repo digest": {
			refs:        []string{"nginx@sha256:aaaa"},
			repoDigests: []string{"nginx@sha256:aaaa"},
			want:        true,
		},
		"untagged image matches nothing by reference": {
			refs:     []string{"nginx"},
			repoTags: []string{"<none>:<none>"},
			want:     false,
		},
		"label key matches any value": {
			labels:      []string{versionLabel},
			repoTags:    []string{"nginx:1.27"},
			imageLabels: map[string]string{versionLabel: "1.27"},
			want:        true,
		},
		"label key matches across versions": {
			labels:      []string{versionLabel},
			repoTags:    []string{"nginx:1.28"},
			imageLabels: map[string]string{versionLabel: "1.28"},
			want:        true,
		},
		"label key and value": {
			labels:      []string{versionLabel + "=1.27"},
			imageLabels: map[string]string{versionLabel: "1.27"},
			want:        true,
		},
		"label value mismatch": {
			labels:      []string{versionLabel + "=1.27"},
			imageLabels: map[string]string{versionLabel: "1.28"},
			want:        false,
		},
		"unlabelled workload image is not pinned": {
			labels:      []string{versionLabel},
			repoTags:    []string{"busybox:1.37"},
			imageLabels: map[string]string{"maintainer": "busybox"},
			want:        false,
		},
		"empty entries are ignored": {
			refs:     []string{"", "   "},
			labels:   []string{"", "  "},
			repoTags: []string{"nginx:1.27"},
			want:     false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			m := newPinnedImageMatcher(tc.refs, tc.labels)
			assert.Equal(
				t,
				tc.want,
				m.matches(tc.repoTags, tc.repoDigests, tc.imageLabels),
			)
		})
	}
}

func TestPinnedImageMatcherIsEmpty(t *testing.T) {
	var nilMatcher *pinnedImageMatcher
	assert.True(t, nilMatcher.isEmpty())
	assert.False(t, nilMatcher.matches([]string{"a:b"}, nil, nil))

	assert.True(t, newPinnedImageMatcher(nil, nil).isEmpty())
	assert.True(t, newPinnedImageMatcher([]string{""}, []string{" "}).isEmpty())
	assert.False(t, newPinnedImageMatcher([]string{"a"}, nil).isEmpty())
	assert.False(t, newPinnedImageMatcher(nil, []string{"k=v"}).isEmpty())
}

func TestRefRepository(t *testing.T) {
	tests := map[string]string{
		"nginx:1.27":                        "nginx",
		"nginx":                             "nginx",
		"localhost:5000/library/nginx:1.27": "localhost:5000/library/nginx",
		"localhost:5000/library/nginx":      "localhost:5000/library/nginx",
		"nginx@sha256:aaaa":                 "nginx",
		"registry.k8s.io/pause:3.10":        "registry.k8s.io/pause",
	}
	for ref, want := range tests {
		assert.Equal(t, want, refRepository(ref), ref)
	}
}
