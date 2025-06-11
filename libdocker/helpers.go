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

package libdocker

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"

	dockerref "github.com/distribution/reference"
	dockertypes "github.com/docker/docker/api/types"
	dockermount "github.com/docker/docker/api/types/mount"
	godigest "github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/cri-api/pkg/apis/runtime/v1"
	crierrors "k8s.io/cri-api/pkg/errors"
	"k8s.io/kubernetes/pkg/apis/core"
)

const windowsEtcHostsPath = "C:\\Windows\\System32\\drivers\\etc\\hosts"

// ParseDockerTimestamp parses the timestamp returned by DockerClientInterface from string to time.Time
func ParseDockerTimestamp(s string) (time.Time, error) {
	// Timestamp returned by Docker is in time.RFC3339Nano format.
	return time.Parse(time.RFC3339Nano, s)
}

// matchImageTagOrSHA checks if the given image specifier is a valid image ref,
// and that it matches the given image. It should fail on things like image IDs
// (config digests) and other digest-only references, but succeed on image names
// (`foo`), tag references (`foo:bar`), and manifest digest references
// (`foo@sha256:xyz`).
func matchImageTagOrSHA(inspected dockertypes.ImageInspect, image string) bool {
	// The image string follows the grammar specified here
	// https://github.com/distribution/reference/blob/master/reference.go#L4
	named, err := dockerref.ParseNormalizedNamed(image)
	if err != nil {
		logrus.Errorf("Couldn't parse image (%s) reference: %v", image, err)
		return false
	}
	_, isTagged := named.(dockerref.Tagged)
	digest, isDigested := named.(dockerref.Digested)
	if !isTagged && !isDigested {
		// No Tag or SHA specified, so just return what we have
		return true
	}

	if isTagged {
		// Check the RepoTags for a match.
		for _, tag := range inspected.RepoTags {
			// An image name (without the tag/digest) can be [hostname '/'] component ['/' component]*
			// Because either the RepoTag or the name *may* contain the
			// hostname or not, we only check for the suffix match.
			if strings.HasSuffix(image, tag) || strings.HasSuffix(tag, image) {
				return true
			} else {
				// docker distro(s) like centos/fedora/rhel image fix problems on
				// their end.
				// Say the tag is "docker.io/busybox:latest"
				// and the image is "docker.io/library/busybox:latest"
				initialName, err := dockerref.ParseNormalizedNamed(tag)
				if err != nil {
					continue
				}
				// the parsed/normalized tag will look like
				// reference.taggedReference {
				// 	 namedRepository: reference.repository {
				// 	   domain: "docker.io",
				// 	   path: "library/busybox"
				//	},
				// 	tag: "latest"
				// }
				// If it does not have tags then we bail out
				maybeTag, ok := initialName.(dockerref.Tagged)
				if !ok {
					continue
				}

				normalizedTag := maybeTag.String()
				if normalizedTag == "" {
					continue
				}
				if strings.HasSuffix(image, normalizedTag) || strings.HasSuffix(normalizedTag, image) {
					return true
				}
			}
		}
	}

	if isDigested {
		for _, repoDigest := range inspected.RepoDigests {
			named, err := dockerref.ParseNormalizedNamed(repoDigest)
			if err != nil {
				logrus.Errorf(
					"Couldn't parse image RepoDigest reference %s: %v",
					repoDigest,
					err,
				)
				continue
			}
			if d, isDigested := named.(dockerref.Digested); isDigested {
				if digest.Digest().Algorithm().String() == d.Digest().Algorithm().String() &&
					digest.Digest().Hex() == d.Digest().Hex() {
					return true
				}
			}
		}

		// process the ID as a digest
		id, err := godigest.Parse(inspected.ID)
		if err != nil {
			logrus.Errorf("Couldn't parse image ID %s reference: %v", id, err)
			return false
		}
		if digest.Digest().Algorithm().String() == id.Algorithm().String() &&
			digest.Digest().Hex() == id.Hex() {
			return true
		}
	}
	logrus.Infof(
		"Inspected image ID %s does not match image %s",
		inspected.ID,
		image,
	)
	return false
}

// matchImageIDOnly checks that the given image specifier is a digest-only
// reference, and that it matches the given image.
func matchImageIDOnly(inspected dockertypes.ImageInspect, image string) bool {
	// If the image ref is literally equal to the inspected image's ID,
	// just return true here (this might be the case for Docker 1.9,
	// where we won't have a digest for the ID)
	if inspected.ID == image {
		return true
	}

	// Otherwise, we should try actual parsing to be more correct
	ref, err := dockerref.Parse(image)
	if err != nil {
		logrus.Infof("Couldn't parse image reference %s: %v", image, err)
		return false
	}

	digest, isDigested := ref.(dockerref.Digested)
	if !isDigested {
		logrus.Infof("The image reference %s was not a digest reference", image)
		return false
	}

	id, err := godigest.Parse(inspected.ID)
	if err != nil {
		logrus.Infof("Couldn't parse image ID reference %s: %v", id, err)
		return false
	}

	if digest.Digest().Algorithm().String() == id.Algorithm().String() &&
		digest.Digest().Hex() == id.Hex() {
		return true
	}

	logrus.Infof("The image reference %s does not directly refer to the given image's ID %s",
		image, inspected.ID)
	return false
}

func CheckContainerStatus(
	client DockerClientInterface,
	containerID string,
) (*dockertypes.ContainerJSON, error) {
	container, err := client.InspectContainer(containerID)
	if err != nil {
		return nil, err
	}
	if !container.State.Running {
		return nil, fmt.Errorf("container not running (%s)", container.ID)
	}
	return container, nil
}

// generateEnvList converts KeyValue list to a list of strings, in the form of
// '<key>=<value>', which can be understood by docker.
func GenerateEnvList(envs []*v1.KeyValue) (result []string) {
	for _, env := range envs {
		result = append(result, fmt.Sprintf("%s=%s", env.Key, env.Value))
	}
	return
}

// generateMountBindings converts the mount list to a list of [dockermount.Mount] that
// can be understood by docker.
// SELinux labels are not handled here.
func GenerateMountBindings(mounts []*v1.Mount, terminationMessagePath string, rtHandler *v1.RuntimeHandler) ([]dockermount.Mount, error) {
	var rroSupported bool
	if rtHandler != nil {
		rroSupported = rtHandler.Features.RecursiveReadOnlyMounts
	}
	if terminationMessagePath == "" {
		terminationMessagePath = core.TerminationMessagePathDefault
	}
	result := make([]dockermount.Mount, 0, len(mounts))
	for _, m := range mounts {
		hostPath, containerPath := m.HostPath, m.ContainerPath
		if runtime.GOOS == "windows" {
			if isSingleFileMount(hostPath, containerPath, terminationMessagePath) {
				logrus.Debugf("skipping mount :%s:%s", m.HostPath, m.ContainerPath)
				continue
			}
			if !isNamedPipe(hostPath) {
				hostPath = filepath.Clean(hostPath)
			}
			if !isNamedPipe(containerPath) {
				containerPath = filepath.Clean(containerPath)
				if containerPath[0] == '\\' {
					containerPath = "C:" + containerPath
				}
			}
		}
		bind := dockermount.Mount{
			Type:   dockermount.TypeBind,
			Source: hostPath,
			Target: containerPath,
			BindOptions: &dockermount.BindOptions{
				CreateMountpoint: true,
			},
		}
		if m.RecursiveReadOnly {
			if !rroSupported {
				return nil, fmt.Errorf("%w: runtime handler does not support recursive read-only mounts (hostPath=%q)",
					crierrors.ErrRROUnsupported, m.HostPath)
			}
			if m.Propagation != v1.MountPropagation_PROPAGATION_PRIVATE {
				return nil, fmt.Errorf("recursive read-only mount needs private propagation, got %q (hostPath=%q)",
					m.Propagation.String(), m.HostPath)
			}
			if !m.Readonly {
				return nil, fmt.Errorf("recursive read-only mount conflicts with RW mount (hostPath=%q)",
					m.HostPath)
			}
		}
		if m.Readonly {
			bind.ReadOnly = true
			// Docker v25 treats read-only mounts as recursively read-only by default,
			// but this appeared to be too much breaking for Kubernetes
			// https://github.com/Mirantis/cri-dockerd/issues/309
			bind.BindOptions.ReadOnlyNonRecursive = !m.RecursiveReadOnly
		}
		switch m.Propagation {
		case v1.MountPropagation_PROPAGATION_PRIVATE:
			// noop, dockerd will decide the propagation.
			//
			// dockerd's default propagation is "rprivate": https://github.com/moby/moby/blob/v20.10.23/volume/mounts/linux_parser.go#L145
			//
			// However, dockerd automatically changes the default propagation to "rslave"
			// when the mount source contains the daemon root (/var/lib/docker):
			// - https://github.com/moby/moby/blob/v20.10.23/daemon/volumes.go#L137-L143
			// - https://github.com/moby/moby/blob/v20.10.23/daemon/volumes_linux.go#L11-L36
			//
			// This behavior was introduced in Docker 18.03: https://github.com/moby/moby/pull/36055
		case v1.MountPropagation_PROPAGATION_BIDIRECTIONAL:
			bind.BindOptions.Propagation = dockermount.PropagationRShared
		case v1.MountPropagation_PROPAGATION_HOST_TO_CONTAINER:
			bind.BindOptions.Propagation = dockermount.PropagationRSlave
		default:
			logrus.Infof("Unknown propagation mode for hostPath %s", m.HostPath)
			// let dockerd decide the propagation
		}

		result = append(result, bind)
	}
	return result, nil
}

func isSingleFileMount(hostPath string, containerPath string, terminationMessagePath string) bool {
	if strings.Contains(containerPath, terminationMessagePath) {
		return true
	}
	if strings.Contains(hostPath, windowsEtcHostsPath) || strings.Contains(containerPath, windowsEtcHostsPath) {
		return true
	}
	return false
}

func MakePortsAndBindings(
	pm []*v1.PortMapping,
) (nat.PortSet, map[nat.Port][]nat.PortBinding) {
	exposedPorts := nat.PortSet{}
	portBindings := map[nat.Port][]nat.PortBinding{}
	for _, port := range pm {
		exteriorPort := port.HostPort
		if exteriorPort == 0 {
			// No need to do port binding when HostPort is not specified
			continue
		}
		interiorPort := port.ContainerPort
		// Some of this port stuff is under-documented voodoo.
		// See http://stackoverflow.com/questions/20428302/binding-a-port-to-a-host-interface-using-the-rest-api
		var protocol string
		switch port.Protocol {
		case v1.Protocol_UDP:
			protocol = "/udp"
		case v1.Protocol_TCP:
			protocol = "/tcp"
		case v1.Protocol_SCTP:
			protocol = "/sctp"
		default:
			logrus.Infof("Unknown protocol %s, defaulting to TCP", port.Protocol)
			protocol = "/tcp"
		}

		dockerPort := nat.Port(strconv.Itoa(int(interiorPort)) + protocol)
		exposedPorts[dockerPort] = struct{}{}

		hostBinding := nat.PortBinding{
			HostPort: strconv.Itoa(int(exteriorPort)),
			HostIP:   port.HostIp,
		}

		// Allow multiple host ports bind to same docker port
		if existedBindings, ok := portBindings[dockerPort]; ok {
			// If a docker port already map to a host port, just append the host ports
			portBindings[dockerPort] = append(existedBindings, hostBinding)
		} else {
			// Otherwise, it's fresh new port binding
			portBindings[dockerPort] = []nat.PortBinding{
				hostBinding,
			}
		}
	}
	return exposedPorts, portBindings
}

func isNamedPipe(s string) bool {
	return strings.HasPrefix(s, `\\.\pipe\`)
}
