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
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Mirantis/cri-dockerd/config"
	"github.com/sirupsen/logrus"

	dockercontainer "github.com/docker/docker/api/types/container"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	knetwork "github.com/Mirantis/cri-dockerd/network"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// applySandboxSecurityContext updates docker sandbox options according to security context.
func applySandboxSecurityContext(
	lc *runtimeapi.LinuxPodSandboxConfig,
	config *dockercontainer.Config,
	hc *dockercontainer.HostConfig,
	network *knetwork.PluginManager,
	separator rune,
) error {
	if lc == nil {
		return nil
	}

	var sc *runtimeapi.LinuxContainerSecurityContext
	if lc.SecurityContext != nil {
		sc = &runtimeapi.LinuxContainerSecurityContext{
			SupplementalGroups: lc.SecurityContext.SupplementalGroups,
			RunAsUser:          lc.SecurityContext.RunAsUser,
			RunAsGroup:         lc.SecurityContext.RunAsGroup,
			ReadonlyRootfs:     lc.SecurityContext.ReadonlyRootfs,
			SelinuxOptions:     lc.SecurityContext.SelinuxOptions,
			NamespaceOptions:   lc.SecurityContext.NamespaceOptions,
			Privileged:         lc.SecurityContext.Privileged,
		}
	}

	err := modifyContainerConfig(sc, config)
	if err != nil {
		return err
	}

	if err := modifyHostConfig(sc, hc, separator); err != nil {
		return err
	}
	modifySandboxNamespaceOptions(sc.GetNamespaceOptions(), hc, network)
	return nil
}

// applyContainerSecurityContext updates docker container options according to security context.
func applyContainerSecurityContext(
	lc *runtimeapi.LinuxContainerConfig,
	podSandboxID string,
	config *dockercontainer.Config,
	hc *dockercontainer.HostConfig,
	separator rune,
) error {
	if lc == nil {
		return nil
	}

	err := modifyContainerConfig(lc.SecurityContext, config)
	if err != nil {
		return err
	}
	if err := modifyHostConfig(lc.SecurityContext, hc, separator); err != nil {
		return err
	}
	modifyContainerNamespaceOptions(lc.SecurityContext.GetNamespaceOptions(), podSandboxID, hc)
	return nil
}

// modifyContainerConfig applies container security context config to dockercontainer.Config.
func modifyContainerConfig(
	sc *runtimeapi.LinuxContainerSecurityContext,
	config *dockercontainer.Config,
) error {
	if sc == nil {
		return nil
	}
	if sc.RunAsUser != nil {
		config.User = strconv.FormatInt(sc.GetRunAsUser().Value, 10)
	}
	if sc.RunAsUsername != "" {
		config.User = sc.RunAsUsername
	}

	user := config.User
	if sc.RunAsGroup != nil {
		if user == "" {
			return fmt.Errorf("runAsGroup is specified without a runAsUser")
		}
		user = fmt.Sprintf("%s:%d", config.User, sc.GetRunAsGroup().Value)
	}

	config.User = user

	return nil
}

// modifyHostConfig applies security context config to dockercontainer.HostConfig.
func modifyHostConfig(
	sc *runtimeapi.LinuxContainerSecurityContext,
	hostConfig *dockercontainer.HostConfig,
	separator rune,
) error {
	if sc == nil {
		return nil
	}

	// Apply supplemental groups.
	for _, group := range sc.SupplementalGroups {
		hostConfig.GroupAdd = append(hostConfig.GroupAdd, strconv.FormatInt(group, 10))
	}

	// Apply security context for the container.
	hostConfig.Privileged = sc.Privileged
	hostConfig.ReadonlyRootfs = sc.ReadonlyRootfs
	if sc.Capabilities != nil {
		hostConfig.CapAdd = sc.GetCapabilities().AddCapabilities
		hostConfig.CapDrop = sc.GetCapabilities().DropCapabilities
	}
	if sc.SelinuxOptions != nil {
		hostConfig.SecurityOpt = addSELinuxOptions(
			hostConfig.SecurityOpt,
			sc.SelinuxOptions,
			separator,
		)
	}

	// Apply apparmor options.
	apparmorSecurityOpts, err := getApparmorSecurityOpts(sc, separator)
	if err != nil {
		return fmt.Errorf("failed to generate apparmor security options: %v", err)
	}
	hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, apparmorSecurityOpts...)

	if sc.NoNewPrivs {
		hostConfig.SecurityOpt = append(hostConfig.SecurityOpt, "no-new-privileges")
	}

	if !hostConfig.Privileged {
		hostConfig.MaskedPaths = sc.MaskedPaths
		hostConfig.ReadonlyPaths = sc.ReadonlyPaths
	}

	return nil
}

// modifySandboxNamespaceOptions apply namespace options for sandbox
func modifySandboxNamespaceOptions(
	nsOpts *runtimeapi.NamespaceOption,
	hostConfig *dockercontainer.HostConfig,
	network *knetwork.PluginManager,
) {
	// The sandbox's PID namespace is the one that's shared, so CONTAINER and POD are equivalent for it
	if nsOpts.GetPid() == runtimeapi.NamespaceMode_NODE {
		hostConfig.PidMode = namespaceModeHost
	}
	modifyHostOptionsForSandbox(nsOpts, network, hostConfig)
}

// modifyContainerNamespaceOptions apply namespace options for container
func modifyContainerNamespaceOptions(
	nsOpts *runtimeapi.NamespaceOption,
	podSandboxID string,
	hostConfig *dockercontainer.HostConfig,
) {
	switch nsOpts.GetPid() {
	case runtimeapi.NamespaceMode_NODE:
		hostConfig.PidMode = namespaceModeHost
	case runtimeapi.NamespaceMode_POD:
		hostConfig.PidMode = dockercontainer.PidMode(fmt.Sprintf("container:%v", podSandboxID))
	case runtimeapi.NamespaceMode_TARGET:
		hostConfig.PidMode = dockercontainer.PidMode(
			fmt.Sprintf("container:%v", nsOpts.GetTargetId()),
		)
	}
	modifyHostOptionsForContainer(nsOpts, podSandboxID, hostConfig)
}

// modifyHostOptionsForSandbox applies NetworkMode/UTSMode to sandbox's dockercontainer.HostConfig.
func modifyHostOptionsForSandbox(
	nsOpts *runtimeapi.NamespaceOption,
	network *knetwork.PluginManager,
	hc *dockercontainer.HostConfig,
) {
	if nsOpts.GetIpc() == runtimeapi.NamespaceMode_NODE {
		hc.IpcMode = namespaceModeHost
	}
	if nsOpts.GetNetwork() == runtimeapi.NamespaceMode_NODE {
		hc.NetworkMode = namespaceModeHost
		return
	}

	if network == nil {
		hc.NetworkMode = "default"
		return
	}

	switch network.PluginName() {
	case "cni":
		fallthrough
	case "kubenet":
		hc.NetworkMode = "none"
	default:
		hc.NetworkMode = "default"
	}
}

// modifyHostOptionsForContainer applies NetworkMode/UTSMode to container's dockercontainer.HostConfig.
func modifyHostOptionsForContainer(
	nsOpts *runtimeapi.NamespaceOption,
	podSandboxID string,
	hc *dockercontainer.HostConfig,
) {
	sandboxNSMode := fmt.Sprintf("container:%v", podSandboxID)
	hc.NetworkMode = dockercontainer.NetworkMode(sandboxNSMode)
	hc.IpcMode = dockercontainer.IpcMode(sandboxNSMode)
	hc.UTSMode = ""

	if nsOpts.GetNetwork() == runtimeapi.NamespaceMode_NODE {
		hc.UTSMode = namespaceModeHost
	}
}

func getSeccompDockerOpts(seccomp *runtimeapi.SecurityProfile, privileged bool) ([]DockerOpt, error) {

	if seccomp == nil || seccomp.GetProfileType() == runtimeapi.SecurityProfile_Unconfined {
		// return early the default
		return defaultSeccompOpt, nil
	}

	if seccomp.GetProfileType() == runtimeapi.SecurityProfile_RuntimeDefault ||
		seccomp.GetProfileType().String() == config.DeprecatedSeccompProfileDockerDefault {
		// return nil so docker will load the default seccomp profile
		return nil, nil
	}

	if seccomp.GetProfileType() != runtimeapi.SecurityProfile_Localhost {
		return nil, fmt.Errorf("unknown seccomp profile option: %s", seccomp)
	}

	// get the full path of seccomp profile when prefixed with 'localhost/'.
	fname := seccomp.GetLocalhostRef()
	if !filepath.IsAbs(fname) {
		return nil, fmt.Errorf(
			"seccomp profile path must be absolute, but got relative path %q",
			fname,
		)
	}
	file, err := os.ReadFile(filepath.FromSlash(fname))
	if err != nil {
		return nil, fmt.Errorf("cannot load seccomp profile %q: %v", fname, err)
	}

	seccompSpec := &specs.LinuxSeccomp{}
	if err := json.Unmarshal(file, seccompSpec); err != nil {
		return nil, fmt.Errorf("decoding seccomp profile failed %q: %v", fname, err)
	}
	// https://github.com/kubernetes-sigs/cri-tools/blob/8869f48d4b120b5f775413b2ca7f8073586d08b4/pkg/validate/security_context_linux.go#L891
	var filteredSyscalls []specs.LinuxSyscall
	for _, scall := range seccompSpec.Syscalls {
		var filteredSyscallNames []string
		for _, name := range scall.Names {
			if privileged && name == "sethostname" && scall.Action == specs.ActErrno {
				logrus.Info("Ignore the seccomp rule that blocks setting hostname when privileged ")
				continue
			}
			filteredSyscallNames = append(filteredSyscallNames, name)
		}
		if len(filteredSyscallNames) == 0 {
			continue
		}
		scall.Names = filteredSyscallNames
		filteredSyscalls = append(filteredSyscalls, scall)
	}
	if len(filteredSyscalls) == 0 {
		return nil, nil
	}
	seccompSpec.Syscalls = filteredSyscalls
	file, err = json.Marshal(seccompSpec)
	if err != nil {
		return nil, fmt.Errorf("re-encoding seccomp profile failed %q: %v", fname, err)
	}

	b := bytes.NewBuffer(nil)
	if err := json.Compact(b, file); err != nil {
		return nil, err
	}
	// Rather than the full profile, just put the filename & md5sum in the event log.
	msg := fmt.Sprintf("%s(md5:%x)", fname, md5.Sum(file))

	return []DockerOpt{{"seccomp", b.String(), msg}}, nil
}

// getSeccompSecurityOpts gets container seccomp options from container seccomp profile.
// It is an experimental feature and may be promoted to official runtime api in the future.
func getSeccompSecurityOpts(seccompProfile *runtimeapi.SecurityProfile, privileged bool, separator rune) ([]string, error) {
	seccompOpts, err := getSeccompDockerOpts(seccompProfile, privileged)
	if err != nil {
		return nil, err
	}
	return FmtDockerOpts(seccompOpts, separator), nil
}

// getApparmorSecurityOpts gets apparmor options from container config.
func getApparmorSecurityOpts(
	sc *runtimeapi.LinuxContainerSecurityContext,
	separator rune,
) ([]string, error) {
	if sc == nil || sc.ApparmorProfile == "" {
		return nil, nil
	}

	appArmorOpts, err := getAppArmorOpts(sc.ApparmorProfile)
	if err != nil {
		return nil, err
	}

	fmtOpts := FmtDockerOpts(appArmorOpts, separator)
	return fmtOpts, nil
}

func getAppArmorOpts(profile string) ([]DockerOpt, error) {
	if profile == "" || profile == config.AppArmorBetaProfileRuntimeDefault {
		// The docker applies the default profile by default.
		return nil, nil
	}

	// Return unconfined profile explicitly
	if profile == config.AppArmorBetaProfileNameUnconfined {
		return []DockerOpt{{"apparmor", config.AppArmorBetaProfileNameUnconfined, ""}}, nil
	}

	// Assume validation has already happened.
	profileName := strings.TrimPrefix(profile, config.AppArmorBetaProfileNamePrefix)
	return []DockerOpt{{"apparmor", profileName, ""}}, nil
}
