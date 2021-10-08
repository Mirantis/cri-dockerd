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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/Mirantis/cri-dockerd/cm"
	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/metrics"
	"github.com/Mirantis/cri-dockerd/network"
	"github.com/Mirantis/cri-dockerd/network/cni"
	"github.com/Mirantis/cri-dockerd/network/hostport"
	"github.com/Mirantis/cri-dockerd/network/kubenet"
	"github.com/Mirantis/cri-dockerd/store"
	"github.com/Mirantis/cri-dockerd/streaming"
	"github.com/Mirantis/cri-dockerd/utils"

	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	dockerRuntimeName = "docker"
	kubeAPIVersion    = "0.1.0"

	// String used to detect docker host mode for various namespaces (e.g.
	// networking). Must match the value returned by docker inspect -f
	// '{{.HostConfig.NetworkMode}}'.
	namespaceModeHost = "host"

	dockerNetNSFmt = "/proc/%v/ns/net"

	// Internal docker labels used to identify whether a container is a sandbox
	// or a regular container.
	containerTypeLabelKey       = "io.kubernetes.docker.type"
	containerTypeLabelSandbox   = "podsandbox"
	containerTypeLabelContainer = "container"
	containerLogPathLabelKey    = "io.kubernetes.container.logpath"
	sandboxIDLabelKey           = "io.kubernetes.sandbox.id"

	// The expiration time of version cache.
	versionCacheTTL = 60 * time.Second
	maxMsgSize      = 1024 * 1024 * 16

	defaultCgroupDriver = "cgroupfs"
)

// CRIService includes all methods necessary for a CRI backend.
type CRIService interface {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
	Start() error
}

// DockerService is an interface that embeds the new RuntimeService and
// ImageService interfaces.
type DockerService interface {
	CRIService

	// For serving streaming calls.
	http.Handler

	// GetContainerLogs gets logs for a specific container.
	GetContainerLogs(
		context.Context,
		*v1.Pod,
		config.ContainerID,
		*v1.PodLogOptions,
		io.Writer,
		io.Writer,
	) error

	// IsCRISupportedLogDriver checks whether the logging driver used by docker is
	// supported by native CRI integration.
	IsCRISupportedLogDriver() (bool, error)

	// Get the last few lines of the logs for a specific container.
	GetContainerLogTail(
		uid config.UID,
		name, namespace string,
		containerID config.ContainerID,
	) (string, error)
}

var internalLabelKeys = []string{containerTypeLabelKey, containerLogPathLabelKey, sandboxIDLabelKey}

// NewDockerService creates a new `DockerService` struct.
// NOTE: Anything passed to DockerService should be eventually handled in another way when we switch to running the shim as a different process.
func NewDockerService(
	clientConfig *config.ClientConfig,
	podSandboxImage string,
	streamingConfig *streaming.Config,
	pluginSettings *config.NetworkPluginSettings,
	cgroupsName string,
	kubeCgroupDriver string,
	criDockerdRootDir string,
) (DockerService, error) {

	client := config.NewDockerClientFromConfig(clientConfig)

	c := libdocker.NewInstrumentedInterface(client)

	checkpointManager, err := store.NewCheckpointManager(
		filepath.Join(criDockerdRootDir, sandboxCheckpointDir),
	)
	if err != nil {
		return nil, err
	}

	ds := &dockerService{
		client:          c,
		os:              config.RealOS{},
		podSandboxImage: podSandboxImage,
		streamingRuntime: &streaming.StreamingRuntime{
			Client:      client,
			ExecHandler: &NativeExecHandler{},
		},
		containerManager:      cm.NewContainerManager(cgroupsName, client),
		checkpointManager:     checkpointManager,
		networkReady:          make(map[string]bool),
		containerCleanupInfos: make(map[string]*containerCleanupInfo),
	}

	// check docker version compatibility.
	if err = ds.checkVersionCompatibility(); err != nil {
		return nil, err
	}

	// create streaming backend if configured.
	if streamingConfig != nil {
		var err error
		ds.streamingServer, err = streaming.NewServer(*streamingConfig, ds.streamingRuntime)
		if err != nil {
			return nil, err
		}
	}

	// Determine the hairpin mode.
	if err := effectiveHairpinMode(pluginSettings); err != nil {
		// This is a non-recoverable error. Returning it up the callstack will just
		// lead to retries of the same failure, so just fail hard.
		return nil, err
	}
	logrus.Info("Hairpin mode is set", "hairpinMode", pluginSettings.HairpinMode)

	// cri-dockerd currently only supports CNI plugins.
	pluginSettings.PluginBinDirs = cni.SplitDirs(pluginSettings.PluginBinDirString)
	cniPlugins := cni.ProbeNetworkPlugins(
		pluginSettings.PluginConfDir,
		pluginSettings.PluginCacheDir,
		pluginSettings.PluginBinDirs,
	)
	cniPlugins = append(
		cniPlugins,
		kubenet.NewPlugin(pluginSettings.PluginBinDirs, pluginSettings.PluginCacheDir),
	)
	netHost := &dockerNetworkHost{
		&namespaceGetter{ds},
		&portMappingGetter{ds},
	}
	plug, err := network.InitNetworkPlugin(
		cniPlugins,
		pluginSettings.PluginName,
		netHost,
		pluginSettings.HairpinMode,
		pluginSettings.NonMasqueradeCIDR,
		pluginSettings.MTU,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"didn't find compatible CNI plugin with given settings %+v: %v",
			pluginSettings,
			err,
		)
	}
	ds.network = network.NewPluginManager(plug)
	logrus.Info(
		"Docker cri networking managed by the network plugin",
		"networkPluginName",
		plug.Name(),
	)

	// skipping cgroup driver checks for Windows
	if runtime.GOOS == "linux" {
		// NOTE: cgroup driver is only detectable in docker 1.11+
		cgroupDriver := defaultCgroupDriver
		dockerInfo, err := ds.client.Info()
		logrus.Info("Docker Info", "dockerInfo", dockerInfo)
		if err != nil {
			logrus.Error(err, "Failed to execute Info() call to the Docker client")
			logrus.Info("Falling back to use the default driver", "cgroupDriver", cgroupDriver)
		} else if len(dockerInfo.CgroupDriver) == 0 {
			logrus.Info("No cgroup driver is set in Docker")
			logrus.Info("Falling back to use the default driver", "cgroupDriver", cgroupDriver)
		} else {
			cgroupDriver = dockerInfo.CgroupDriver
		}
		if len(kubeCgroupDriver) != 0 && kubeCgroupDriver != cgroupDriver {
			return nil, fmt.Errorf(
				"misconfiguration: kubelet cgroup driver: %q is different from docker cgroup driver: %q",
				kubeCgroupDriver,
				cgroupDriver,
			)
		}
		logrus.Info("Setting cgroupDriver", "cgroupDriver", cgroupDriver)
		ds.cgroupDriver = cgroupDriver
	}

	ds.versionCache = store.NewObjectCache(
		func() (interface{}, error) {
			return ds.getDockerVersion()
		},
		versionCacheTTL,
	)

	// Register prometheus metrics.
	metrics.Register()

	return ds, nil
}

type dockerService struct {
	client           libdocker.DockerClientInterface
	os               config.OSInterface
	podSandboxImage  string
	streamingRuntime *streaming.StreamingRuntime
	streamingServer  streaming.Server

	network *network.PluginManager
	// Map of podSandboxID :: network-is-ready
	networkReady     map[string]bool
	networkReadyLock sync.Mutex

	containerManager cm.ContainerManager
	// cgroup driver used by Docker runtime.
	cgroupDriver      string
	checkpointManager store.CheckpointManager
	// caches the version of the runtime.
	// To be compatible with multiple docker versions, we need to perform
	// version checking for some operations. Use this cache to avoid querying
	// the docker daemon every time we need to do such checks.
	versionCache *store.ObjectCache

	// containerCleanupInfos maps container IDs to the `containerCleanupInfo` structs
	// needed to clean up after containers have been removed.
	// (see `applyPlatformSpecificDockerConfig` and `performPlatformSpecificContainerCleanup`
	// methods for more info).
	containerCleanupInfos map[string]*containerCleanupInfo
	cleanupInfosLock      sync.RWMutex
}

// Version returns the runtime name, runtime version and runtime API version
func (ds *dockerService) Version(
	_ context.Context,
	r *runtimeapi.VersionRequest,
) (*runtimeapi.VersionResponse, error) {
	v, err := ds.getDockerVersion()
	if err != nil {
		return nil, err
	}
	return &runtimeapi.VersionResponse{
		Version:           kubeAPIVersion,
		RuntimeName:       dockerRuntimeName,
		RuntimeVersion:    v.Version,
		RuntimeApiVersion: v.APIVersion,
	}, nil
}

// getDockerVersion gets the version information from docker.
func (ds *dockerService) getDockerVersion() (*dockertypes.Version, error) {
	v, err := ds.client.Version()
	if err != nil {
		return nil, fmt.Errorf("failed to get docker version: %v", err)
	}
	// Docker API version (e.g., 1.23) is not semver compatible. Add a ".0"
	// suffix to remedy this.
	v.APIVersion = fmt.Sprintf("%s.0", v.APIVersion)
	return v, nil
}

// UpdateRuntimeConfig updates the runtime config. Currently only handles podCIDR updates.
func (ds *dockerService) UpdateRuntimeConfig(
	_ context.Context,
	r *runtimeapi.UpdateRuntimeConfigRequest,
) (*runtimeapi.UpdateRuntimeConfigResponse, error) {
	runtimeConfig := r.GetRuntimeConfig()
	if runtimeConfig == nil {
		return &runtimeapi.UpdateRuntimeConfigResponse{}, nil
	}

	logrus.Info("Docker cri received runtime config", "runtimeConfig", runtimeConfig)
	if ds.network != nil && runtimeConfig.NetworkConfig.PodCidr != "" {
		event := make(map[string]interface{})
		event[network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE_DETAIL_CIDR] = runtimeConfig.NetworkConfig.PodCidr
		ds.network.Event(network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE, event)
	}

	return &runtimeapi.UpdateRuntimeConfigResponse{}, nil
}

// GetNetNS returns the network namespace of the given containerID. The ID
// supplied is typically the ID of a pod sandbox. This getter doesn't try
// to map non-sandbox IDs to their respective sandboxes.
func (ds *dockerService) GetNetNS(podSandboxID string) (string, error) {
	r, err := ds.client.InspectContainer(podSandboxID)
	if err != nil {
		return "", err
	}
	return getNetworkNamespace(r)
}

// GetPodPortMappings returns the port mappings of the given podSandbox ID.
func (ds *dockerService) GetPodPortMappings(podSandboxID string) ([]*hostport.PortMapping, error) {
	checkpoint := NewPodSandboxCheckpoint("", "", &CheckpointData{})
	err := ds.checkpointManager.GetCheckpoint(podSandboxID, checkpoint)
	// Return empty portMappings if checkpoint is not found
	if err != nil {
		if err == store.ErrCheckpointNotFound {
			return nil, nil
		}
		errRem := ds.checkpointManager.RemoveCheckpoint(podSandboxID)
		if errRem != nil {
			logrus.Error(
				errRem,
				"Failed to delete corrupt checkpoint for sandbox",
				"podSandboxID",
				podSandboxID,
			)
		}
		return nil, err
	}
	_, _, _, checkpointedPortMappings, _ := checkpoint.GetData()
	portMappings := make([]*hostport.PortMapping, 0, len(checkpointedPortMappings))
	for _, pm := range checkpointedPortMappings {
		proto := toProtocol(*pm.Protocol)
		portMappings = append(portMappings, &hostport.PortMapping{
			HostPort:      *pm.HostPort,
			ContainerPort: *pm.ContainerPort,
			Protocol:      proto,
			HostIP:        pm.HostIP,
		})
	}
	return portMappings, nil
}

// Start initializes and starts components in dockerService.
func (ds *dockerService) Start() error {
	ds.initCleanup()

	go func() {
		if err := ds.streamingServer.Start(true); err != nil {
			logrus.Error(err, "Streaming backend stopped unexpectedly")
			os.Exit(1)
		}
	}()

	return ds.containerManager.Start()
}

// initCleanup is responsible for cleaning up any crufts left by previous
// runs. If there are any errors, it simply logs them.
func (ds *dockerService) initCleanup() {
	errors := ds.platformSpecificContainerInitCleanup()

	for _, err := range errors {
		logrus.Info("Initialization error", "err", err)
	}
}

// Status returns the status of the runtime.
func (ds *dockerService) Status(
	_ context.Context,
	r *runtimeapi.StatusRequest,
) (*runtimeapi.StatusResponse, error) {
	runtimeReady := &runtimeapi.RuntimeCondition{
		Type:   runtimeapi.RuntimeReady,
		Status: true,
	}
	networkReady := &runtimeapi.RuntimeCondition{
		Type:   runtimeapi.NetworkReady,
		Status: true,
	}
	conditions := []*runtimeapi.RuntimeCondition{runtimeReady, networkReady}
	if _, err := ds.client.Version(); err != nil {
		runtimeReady.Status = false
		runtimeReady.Reason = "DockerDaemonNotReady"
		runtimeReady.Message = fmt.Sprintf("docker: failed to get docker version: %v", err)
	}
	if err := ds.network.Status(); err != nil {
		networkReady.Status = false
		networkReady.Reason = "NetworkPluginNotReady"
		networkReady.Message = fmt.Sprintf("docker: network plugin is not ready: %v", err)
	}
	status := &runtimeapi.RuntimeStatus{Conditions: conditions}
	return &runtimeapi.StatusResponse{Status: status}, nil
}

func (ds *dockerService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ds.streamingServer != nil {
		ds.streamingServer.ServeHTTP(w, r)
	} else {
		http.NotFound(w, r)
	}
}

// GenerateExpectedCgroupParent returns cgroup parent in syntax expected by cgroup driver
func (ds *dockerService) GenerateExpectedCgroupParent(cgroupParent string) (string, error) {
	if cgroupParent != "" {
		// if docker uses the systemd cgroup driver, it expects *.slice style names for cgroup parent.
		// if we configured kubelet to use --cgroup-driver=cgroupfs, and docker is configured to use systemd driver
		// docker will fail to launch the container because the name we provide will not be a valid slice.
		// this is a very good thing.
		if ds.cgroupDriver == "systemd" {
			// Pass only the last component of the cgroup path to systemd.
			cgroupParent = path.Base(cgroupParent)
		}
	}
	logrus.Info("Setting cgroup parent", "cgroupParent", cgroupParent)
	return cgroupParent, nil
}

// checkVersionCompatibility verifies whether docker is in a compatible version.
func (ds *dockerService) checkVersionCompatibility() error {
	apiVersion, err := ds.getDockerAPIVersion()
	if err != nil {
		return err
	}

	minAPIVersion, err := semver.Parse(libdocker.MinimumDockerAPIVersion)
	if err != nil {
		return err
	}

	// Verify the docker version.
	result := apiVersion.Compare(minAPIVersion)
	if result < 0 {
		return fmt.Errorf("docker API version is older than %s", libdocker.MinimumDockerAPIVersion)
	}

	return nil
}

// getDockerAPIVersion gets the semver-compatible docker api version.
func (ds *dockerService) getDockerAPIVersion() (*semver.Version, error) {
	var dv *dockertypes.Version
	var err error
	if ds.versionCache != nil {
		dv, err = ds.getDockerVersionFromCache()
	} else {
		dv, err = ds.getDockerVersion()
	}
	if err != nil {
		return nil, err
	}

	apiVersion, err := semver.Parse(dv.APIVersion)
	if err != nil {
		return nil, err
	}
	return &apiVersion, nil
}

func (ds *dockerService) getDockerVersionFromCache() (*dockertypes.Version, error) {
	// We only store on key in the cache.
	const dummyKey = "version"
	value, err := ds.versionCache.Get(dummyKey)
	if err != nil {
		return nil, err
	}
	dv, ok := value.(*dockertypes.Version)
	if !ok {
		return nil, fmt.Errorf("converted to *dockertype.Version error")
	}
	return dv, nil
}

// namespaceGetter is a wrapper around the dockerService that implements
// the network.NamespaceGetter interface.
type namespaceGetter struct {
	ds *dockerService
}

func (n *namespaceGetter) GetNetNS(containerID string) (string, error) {
	return n.ds.GetNetNS(containerID)
}

// portMappingGetter is a wrapper around the dockerService that implements
// the network.PortMappingGetter interface.
type portMappingGetter struct {
	ds *dockerService
}

func (p *portMappingGetter) GetPodPortMappings(
	containerID string,
) ([]*hostport.PortMapping, error) {
	return p.ds.GetPodPortMappings(containerID)
}

// dockerNetworkHost implements network.Host by wrapping the legacy host passed in by the kubelet
// and dockerServices which implements the rest of the network host interfaces.
// The legacy host methods are slated for deletion.
type dockerNetworkHost struct {
	*namespaceGetter
	*portMappingGetter
}

func toProtocol(protocol config.Protocol) config.Protocol {
	switch protocol {
	case protocolTCP:
		return config.ProtocolTCP
	case protocolUDP:
		return config.ProtocolUDP
	case protocolSCTP:
		return config.ProtocolSCTP
	}
	logrus.Info("Unknown protocol, defaulting to TCP", "protocol", protocol)
	return config.ProtocolTCP
}

// effectiveHairpinMode determines the effective hairpin mode given the
// configured mode, and whether cbr0 should be configured.
func effectiveHairpinMode(s *config.NetworkPluginSettings) error {
	// The hairpin mode setting doesn't matter if:
	// - We're not using a bridge network. This is hard to check because we might
	//   be using a plugin.
	// - It's set to hairpin-veth for a container runtime that doesn't know how
	//   to set the hairpin flag on the veth's of containers. Currently the
	//   docker runtime is the only one that understands this.
	// - It's set to "none".
	if s.HairpinMode == config.PromiscuousBridge ||
		s.HairpinMode == config.HairpinVeth {
		if s.HairpinMode == config.PromiscuousBridge && s.PluginName != "kubenet" {
			// This is not a valid combination, since promiscuous-bridge only works on kubenet. Users might be using the
			// default values (from before the hairpin-mode flag existed) and we
			// should keep the old behavior.
			logrus.Info(
				"Hairpin mode is set but kubenet is not enabled, falling back to HairpinVeth",
				"hairpinMode",
				s.HairpinMode,
			)
			s.HairpinMode = config.HairpinVeth
			return nil
		}
	} else if s.HairpinMode != config.HairpinNone {
		return fmt.Errorf("unknown value: %q", s.HairpinMode)
	}
	return nil
}

// ExecSync executes a command in the container, and returns the stdout output.
// If command exits with a non-zero exit code, an error is returned.
func (ds *dockerService) ExecSync(
	ctx context.Context,
	req *runtimeapi.ExecSyncRequest,
) (*runtimeapi.ExecSyncResponse, error) {
	timeout := time.Duration(req.Timeout) * time.Second
	var stdoutBuffer, stderrBuffer bytes.Buffer
	err := ds.streamingRuntime.ExecWithContext(ctx, req.ContainerId, req.Cmd,
		nil, // in
		utils.WriteCloserWrapper(utils.LimitWriter(&stdoutBuffer, maxMsgSize)),
		utils.WriteCloserWrapper(utils.LimitWriter(&stderrBuffer, maxMsgSize)),
		false, // tty
		nil,   // resize
		timeout)

	// kubelet's backend runtime expects a grpc error with status code DeadlineExceeded on time out.
	if err == context.DeadlineExceeded {
		return nil, fmt.Errorf(string(codes.DeadlineExceeded), err.Error())
	}

	var exitCode int32
	if err != nil {
		exitError, ok := err.(utils.ExitError)
		if !ok {
			return nil, err
		}

		exitCode = int32(exitError.ExitStatus())
	}
	return &runtimeapi.ExecSyncResponse{
		Stdout:   stdoutBuffer.Bytes(),
		Stderr:   stderrBuffer.Bytes(),
		ExitCode: exitCode,
	}, nil
}

// Exec prepares a streaming endpoint to execute a command in the container, and returns the address.
func (ds *dockerService) Exec(
	_ context.Context,
	req *runtimeapi.ExecRequest,
) (*runtimeapi.ExecResponse, error) {
	if ds.streamingServer == nil {
		return nil, streaming.NewErrorStreamingDisabled("exec")
	}
	_, err := libdocker.CheckContainerStatus(ds.client, req.ContainerId)
	if err != nil {
		return nil, err
	}
	return ds.streamingServer.GetExec(req)
}

// Attach prepares a streaming endpoint to attach to a running container, and returns the address.
func (ds *dockerService) Attach(
	_ context.Context,
	req *runtimeapi.AttachRequest,
) (*runtimeapi.AttachResponse, error) {
	if ds.streamingServer == nil {
		return nil, streaming.NewErrorStreamingDisabled("attach")
	}
	_, err := libdocker.CheckContainerStatus(ds.client, req.ContainerId)
	if err != nil {
		return nil, err
	}
	return ds.streamingServer.GetAttach(req)
}

// PortForward prepares a streaming endpoint to forward ports from a PodSandbox, and returns the address.
func (ds *dockerService) PortForward(
	_ context.Context,
	req *runtimeapi.PortForwardRequest,
) (*runtimeapi.PortForwardResponse, error) {
	if ds.streamingServer == nil {
		return nil, streaming.NewErrorStreamingDisabled("port forward")
	}
	_, err := libdocker.CheckContainerStatus(ds.client, req.PodSandboxId)
	if err != nil {
		return nil, err
	}
	return ds.streamingServer.GetPortForward(req)
}
