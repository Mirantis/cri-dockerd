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

import "strings"

// pinnedImageMatcher decides whether a locally present image must be reported
// to the kubelet as pinned.
//
// The kubelet drops pinned images from its eviction list altogether, so
// pinning is the supported way to keep an image that cannot simply be pulled
// again -- a platform image side-loaded onto an air-gapped node, say -- from
// being deleted once the node crosses its image GC threshold.
//
// The zero value and a nil matcher match nothing, so callers need no nil check.
type pinnedImageMatcher struct {
	// refs holds fully qualified references ("repo:tag" or "repo@digest"),
	// compared against an image's RepoTags and RepoDigests.
	refs map[string]struct{}
	// repos holds bare repositories, which match any tag of that repository.
	repos map[string]struct{}
	// prefixes holds the leading portion of patterns written with a trailing
	// "*", compared against RepoTags and RepoDigests.
	prefixes []string
	// labels holds selectors applied to an image's labels.
	labels []labelSelector
}

// labelSelector matches an image label by key, and additionally by value when
// the selector was given in "key=value" form.
type labelSelector struct {
	key string
	// value is only consulted when matchAny is false.
	value    string
	matchAny bool
}

// newPinnedImageMatcher builds a matcher from the operator-supplied image
// references and label selectors. Blank entries are ignored so that an unset
// flag, or one holding an empty string, is equivalent to not passing it.
func newPinnedImageMatcher(refs, labels []string) *pinnedImageMatcher {
	m := &pinnedImageMatcher{
		refs:  make(map[string]struct{}),
		repos: make(map[string]struct{}),
	}

	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		switch {
		case ref == "":
		case strings.HasSuffix(ref, "*"):
			m.prefixes = append(m.prefixes, strings.TrimSuffix(ref, "*"))
		case hasTagOrDigest(ref):
			m.refs[ref] = struct{}{}
		default:
			m.repos[ref] = struct{}{}
		}
	}

	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" {
			continue
		}
		key, value, hasValue := strings.Cut(label, "=")
		m.labels = append(m.labels, labelSelector{
			key:      key,
			value:    value,
			matchAny: !hasValue,
		})
	}

	return m
}

// isEmpty reports whether the matcher can never match, which lets the caller
// skip logging about pinning on a default configuration.
func (m *pinnedImageMatcher) isEmpty() bool {
	return m == nil ||
		len(m.refs)+len(m.repos)+len(m.prefixes)+len(m.labels) == 0
}

// matches reports whether an image carrying the given references and labels
// should be pinned.
func (m *pinnedImageMatcher) matches(repoTags, repoDigests []string, labels map[string]string) bool {
	if m == nil {
		return false
	}

	for _, sel := range m.labels {
		value, ok := labels[sel.key]
		if ok && (sel.matchAny || value == sel.value) {
			return true
		}
	}

	for _, refs := range [][]string{repoTags, repoDigests} {
		for _, ref := range refs {
			if m.matchesRef(ref) {
				return true
			}
		}
	}

	return false
}

func (m *pinnedImageMatcher) matchesRef(ref string) bool {
	if _, ok := m.refs[ref]; ok {
		return true
	}
	if _, ok := m.repos[refRepository(ref)]; ok {
		return true
	}
	for _, prefix := range m.prefixes {
		if strings.HasPrefix(ref, prefix) {
			return true
		}
	}
	return false
}

// hasTagOrDigest reports whether a reference carries a tag or a digest. The
// colon of a registry port ("localhost:5000/library/nginx") is not a tag, so only
// the portion after the final slash is considered.
func hasTagOrDigest(ref string) bool {
	if strings.Contains(ref, "@") {
		return true
	}
	return strings.Contains(ref[strings.LastIndex(ref, "/")+1:], ":")
}

// refRepository strips the tag or digest from an image reference, returning it
// unchanged when it carries neither.
func refRepository(ref string) string {
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	nameStart := strings.LastIndex(ref, "/") + 1
	if i := strings.LastIndex(ref[nameStart:], ":"); i >= 0 {
		ref = ref[:nameStart+i]
	}
	return ref
}
