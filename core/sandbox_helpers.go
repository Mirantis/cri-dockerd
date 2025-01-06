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
	"os"
	"strings"
	"time"

	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/utils"
	"github.com/Mirantis/cri-dockerd/utils/errors"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"github.com/Mirantis/cri-dockerd/config"
	dockertypes "github.com/docker/docker/api/types"
	dockerbackend "github.com/docker/docker/api/types/backend"
	dockercontainer "github.com/docker/docker/api/types/container"
	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	defaultSandboxImage = "registry.k8s.io/pause:3.10"

	// Various default sandbox resources requests/limits.
	defaultSandboxCPUshares int64 = 2

	// defaultSandboxOOMAdj is the oom score adjustment for the docker
	// sandbox container. Using this OOM adj makes it very unlikely, but not
	// impossible, that the defaultSandox will experience an oom kill. -998
	// is chosen to signify sandbox should be OOM killed before other more
	// vital processes like the docker daemon, the kubelet, etc...
	defaultSandboxOOMAdj int = -998

	// Name of the underlying container runtime
	runtimeName = "docker"
)

var (
	// Termination grace period
	defaultSandboxGracePeriod = time.Duration(10) * time.Second
)

// check Runtime correct
func (ds *dockerService) IsRuntimeConfigured(runtime string) error {
	info, err := ds.getDockerInfo()
	if err != nil {
		return fmt.Errorf("failed to get docker info: %v", err)
	}

	// ds.runtimeInfoLock.RLock()
	for r := range info.Runtimes {
		if r == runtime {
			return nil
		}
	}
	// ds.runtimeInfoLock.RUnlock()

	return fmt.Errorf("no runtime for %q is configured", runtime)
}

// Returns whether the sandbox network is ready, and whether the sandbox is known
func (ds *dockerService) getNetworkReady(podSandboxID string) (bool, bool) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	ready, ok := ds.networkReady[podSandboxID]
	return ready, ok
}

func (ds *dockerService) setNetworkReady(podSandboxID string, ready bool) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	ds.networkReady[podSandboxID] = ready
}

func (ds *dockerService) clearNetworkReady(podSandboxID string) {
	ds.networkReadyLock.Lock()
	defer ds.networkReadyLock.Unlock()
	delete(ds.networkReady, podSandboxID)
}

// getIPsFromPlugin interrogates the network plugin for sandbox IPs.
func (ds *dockerService) getIPsFromPlugin(sandbox *dockertypes.ContainerJSON) ([]string, error) {
	metadata, err := parseSandboxName(sandbox.Name)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf(
		"Couldn't find network status for %s/%s through plugin",
		metadata.Namespace,
		metadata.Name,
	)
	cID := config.BuildContainerID(runtimeName, sandbox.ID)
	networkStatus, err := ds.network.GetPodNetworkStatus(metadata.Namespace, metadata.Name, cID)
	if err != nil {
		return nil, err
	}
	if networkStatus == nil {
		return nil, fmt.Errorf("%v: invalid network status for", msg)
	}

	ips := make([]string, 0)
	for _, ip := range networkStatus.IPs {
		ips = append(ips, ip.String())
	}
	// if we don't have any ip in our list then cni is using classic primary IP only
	if len(ips) == 0 {
		ips = append(ips, networkStatus.IP.String())
	}
	return ips, nil
}

// getIPs returns the ip given the output of `docker inspect` on a pod sandbox,
// first interrogating any registered plugins, then simply trusting the ip
// in the sandbox itself. We look for an ipv4 address before ipv6.
func (ds *dockerService) getIPs(podSandboxID string, sandbox *dockertypes.ContainerJSON) []string {
	if sandbox.NetworkSettings == nil {
		return nil
	}
	if networkNamespaceMode(sandbox) == runtimeapi.NamespaceMode_NODE {
		// For sandboxes using host network, the shim is not responsible for
		// reporting the IP.
		return nil
	}

	// Don't bother getting IP if the pod is known and networking isn't ready
	ready, ok := ds.getNetworkReady(podSandboxID)
	if ok && !ready {
		return nil
	}

	ips, err := ds.getIPsFromPlugin(sandbox)
	if err == nil {
		return ips
	}

	ips = make([]string, 0)
	// eth0 by default and so does CNI, so if we find a docker IP here, we
	// conclude that the plugin must have failed setup, or forgotten its ip.
	// This is not a sensible assumption for plugins across the board, but if
	// a plugin doesn't want this behavior, it can throw an error.
	if sandbox.NetworkSettings.IPAddress != "" {
		ips = append(ips, sandbox.NetworkSettings.IPAddress)
	}
	if sandbox.NetworkSettings.GlobalIPv6Address != "" {
		ips = append(ips, sandbox.NetworkSettings.GlobalIPv6Address)
	}

	// If all else fails, warn but don't return an error, as pod status
	// should generally not return anything except fatal errors
	logrus.Infof("Failed to read pod IP from plugin/docker: %v", err)
	return ips
}

// Returns the inspect container response, the sandbox metadata, and network namespace mode
func (ds *dockerService) getPodSandboxDetails(
	podSandboxID string,
) (*dockertypes.ContainerJSON, *runtimeapi.PodSandboxMetadata, error) {
	resp, err := ds.client.InspectContainer(podSandboxID)
	if err != nil {
		return nil, nil, err
	}

	metadata, err := parseSandboxName(resp.Name)
	if err != nil {
		return nil, nil, err
	}

	return resp, metadata, nil
}

// applySandboxLinuxOptions applies LinuxPodSandboxConfig to dockercontainer.HostConfig and dockercontainer.ContainerCreateConfig.
func (ds *dockerService) applySandboxLinuxOptions(
	hc *dockercontainer.HostConfig,
	lc *runtimeapi.LinuxPodSandboxConfig,
	createConfig *dockerbackend.ContainerCreateConfig,
	image string,
	separator rune,
) error {
	if lc == nil {
		return nil
	}
	// Apply security context.
	if err := applySandboxSecurityContext(lc, createConfig.Config, hc, ds.network, separator); err != nil {
		return err
	}

	// Set sysctls.
	hc.Sysctls = lc.Sysctls
	return nil
}

func (ds *dockerService) applySandboxResources(
	hc *dockercontainer.HostConfig,
	lc *runtimeapi.LinuxPodSandboxConfig,
) error {
	hc.Resources = dockercontainer.Resources{
		MemorySwap: DefaultMemorySwap(),
		CPUShares:  defaultSandboxCPUshares,
		// Use docker's default cpu quota/period.
	}

	if lc != nil {
		// Apply Cgroup options.
		cgroupParent, err := ds.GenerateExpectedCgroupParent(lc.CgroupParent)
		if err != nil {
			return err
		}
		hc.CgroupParent = cgroupParent
	}
	return nil
}

// makeSandboxDockerConfig returns dockerbackend.ContainerCreateConfig based on runtimeapi.PodSandboxConfig.
func (ds *dockerService) makeSandboxDockerConfig(
	c *runtimeapi.PodSandboxConfig,
	image string,
) (*dockerbackend.ContainerCreateConfig, error) {
	// Merge annotations and labels because docker supports only labels.
	labels := makeLabels(c.GetLabels(), c.GetAnnotations())
	// Apply a label to distinguish sandboxes from regular containers.
	labels[containerTypeLabelKey] = containerTypeLabelSandbox
	// Apply a container name label for infra container. This is used in summary v1.
	labels[config.KubernetesContainerNameLabel] = sandboxContainerName

	hc := &dockercontainer.HostConfig{
		IpcMode: dockercontainer.IpcMode("shareable"),
	}
	createConfig := &dockerbackend.ContainerCreateConfig{
		Name: makeSandboxName(c),
		Config: &dockercontainer.Config{
			Hostname: c.Hostname,
			Image:    image,
			Labels:   labels,
		},
		HostConfig: hc,
	}

	// Apply linux-specific options.
	if err := ds.applySandboxLinuxOptions(hc, c.GetLinux(), createConfig, image, securityOptSeparator); err != nil {
		return nil, err
	}

	// Set port mappings.
	exposedPorts, portBindings := libdocker.MakePortsAndBindings(c.GetPortMappings())
	createConfig.Config.ExposedPorts = exposedPorts
	hc.PortBindings = portBindings

	hc.OomScoreAdj = defaultSandboxOOMAdj

	// Apply resource options.
	if err := ds.applySandboxResources(hc, c.GetLinux()); err != nil {
		return nil, err
	}

	// Set security options.
	securityOpts := ds.getSandBoxSecurityOpts(securityOptSeparator)
	hc.SecurityOpt = append(hc.SecurityOpt, securityOpts...)

	return createConfig, nil
}

// networkNamespaceMode returns the network runtimeapi.NamespaceMode for this container.
// Supports: POD, NODE
func networkNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil &&
		string(container.HostConfig.NetworkMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

// pidNamespaceMode returns the PID runtimeapi.NamespaceMode for this container.
// Supports: CONTAINER, NODE
func pidNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil &&
		string(container.HostConfig.PidMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_CONTAINER
}

// ipcNamespaceMode returns the IPC runtimeapi.NamespaceMode for this container.
// Supports: POD, NODE
func ipcNamespaceMode(container *dockertypes.ContainerJSON) runtimeapi.NamespaceMode {
	if container != nil && container.HostConfig != nil &&
		string(container.HostConfig.IpcMode) == namespaceModeHost {
		return runtimeapi.NamespaceMode_NODE
	}
	return runtimeapi.NamespaceMode_POD
}

func constructPodSandboxCheckpoint(
	sandboxConfig *runtimeapi.PodSandboxConfig,
) Checkpoint {
	data := CheckpointData{}
	for _, pm := range sandboxConfig.GetPortMappings() {
		proto := toCheckpointProtocol(pm.Protocol)
		data.PortMappings = append(data.PortMappings, &config.PortMapping{
			HostPort:      &pm.HostPort,
			ContainerPort: &pm.ContainerPort,
			Protocol:      &proto,
			HostIP:        pm.HostIp,
		})
	}
	if sandboxConfig.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == runtimeapi.NamespaceMode_NODE {
		data.HostNetwork = true
	}
	return NewPodSandboxCheckpoint(sandboxConfig.Metadata.Namespace, sandboxConfig.Metadata.Name, &data)
}

func toCheckpointProtocol(protocol runtimeapi.Protocol) config.Protocol {
	switch protocol {
	case runtimeapi.Protocol_TCP:
		return protocolTCP
	case runtimeapi.Protocol_UDP:
		return protocolUDP
	case runtimeapi.Protocol_SCTP:
		return protocolSCTP
	}
	logrus.Infof("Unknown protocol, defaulting to TCP: %v", protocol)
	return protocolTCP
}

// rewriteResolvFile rewrites resolv.conf file generated by docker.
func rewriteResolvFile(
	resolvFilePath string,
	dns []string,
	dnsSearch []string,
	dnsOptions []string,
) error {
	if len(resolvFilePath) == 0 {
		logrus.Error("ResolvConfPath is empty.")
		return nil
	}

	if _, err := os.Stat(resolvFilePath); os.IsNotExist(err) {
		return fmt.Errorf("ResolvConfPath %q does not exist", resolvFilePath)
	}

	var resolvFileContent []string
	for _, srv := range dns {
		resolvFileContent = append(resolvFileContent, "nameserver "+srv)
	}

	if len(dnsSearch) > 0 {
		resolvFileContent = append(resolvFileContent, "search "+strings.Join(dnsSearch, " "))
	}

	if len(dnsOptions) > 0 {
		resolvFileContent = append(resolvFileContent, "options "+strings.Join(dnsOptions, " "))
	}

	if len(resolvFileContent) > 0 {
		resolvFileContentStr := strings.Join(resolvFileContent, "\n")
		resolvFileContentStr += "\n"

		logrus.Infof("Will attempt to re-write config file %s as %v", resolvFilePath, resolvFileContent)
		if err := rewriteFile(resolvFilePath, resolvFileContentStr); err != nil {
			logrus.Errorf("Resolv.conf could not be updated: %v", err)
			return err
		}
	}

	return nil
}

func rewriteFile(filePath, stringToWrite string) error {
	f, err := os.OpenFile(filePath, os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(stringToWrite)
	return err
}

func recoverFromCreationConflictIfNeeded(
	client libdocker.DockerClientInterface,
	createConfig dockerbackend.ContainerCreateConfig,
	err error,
) (*dockercontainer.CreateResponse, error) {
	matches := conflictRE.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return nil, err
	}

	id := matches[1]
	logrus.Infof("Unable to create pod sandbox due to conflict. Attempting to remove sandbox. Container %v", id)
	rmErr := client.RemoveContainer(id, dockercontainer.RemoveOptions{RemoveVolumes: true})
	if rmErr == nil {
		logrus.Infof("Successfully removed conflicting container: %v", id)
		return nil, err
	}
	logrus.Errorf("Failed to remove the conflicting container (%v): %v", id, rmErr)
	// Return if the error is not container not found error.
	if !libdocker.IsContainerNotFoundError(rmErr) {
		return nil, err
	}

	// randomize the name to avoid conflict.
	createConfig.Name = randomizeName(createConfig.Name)
	logrus.Debugf("Creating a container with a randomized name: %s", createConfig.Name)
	return client.CreateContainer(createConfig)
}

// ensureSandboxImageExists pulls the sandbox image when it's not present.
func ensureSandboxImageExists(client libdocker.DockerClientInterface, image string) error {
	_, err := client.InspectImageByRef(image)
	if err == nil {
		return nil
	}
	if !libdocker.IsImageNotFoundError(err) {
		return fmt.Errorf("failed to inspect sandbox image %q: %v", image, err)
	}

	repoToPull, _, _, err := utils.ParseImageName(image)
	if err != nil {
		return err
	}

	keyring := credentialprovider.NewDockerKeyring()
	creds, withCredentials := keyring.Lookup(repoToPull)
	if !withCredentials {
		logrus.Infof("Pulling the image without credentials. Image: %v", image)

		err := client.PullImage(image, dockerregistry.AuthConfig{}, dockerimage.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed pulling image %q: %v", image, err)
		}

		return nil
	}

	var pullErrs []error
	for _, currentCreds := range creds {
		authConfig := dockerregistry.AuthConfig(currentCreds)
		err := client.PullImage(image, authConfig, dockerimage.PullOptions{})
		// If there was no error, return success
		if err == nil {
			return nil
		}

		pullErrs = append(pullErrs, err)
	}

	return errors.NewAggregate(pullErrs)
}
