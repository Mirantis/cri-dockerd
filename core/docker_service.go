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
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/containermanager"
	"github.com/Mirantis/cri-dockerd/libdocker"
	"github.com/Mirantis/cri-dockerd/metrics"
	"github.com/Mirantis/cri-dockerd/network"
	"github.com/Mirantis/cri-dockerd/network/cni"
	"github.com/Mirantis/cri-dockerd/network/kubenet"
	"github.com/Mirantis/cri-dockerd/store"
	"github.com/Mirantis/cri-dockerd/streaming"
	"github.com/Mirantis/cri-dockerd/utils"
	"github.com/blang/semver"
	dockertypes "github.com/docker/docker/api/types"
	dockersystem "github.com/docker/docker/api/types/system"
	ociruntimefeatures "github.com/opencontainers/runtime-spec/specs-go/features"
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

	systemInfoCacheMinTTL = time.Minute

	maxMsgSize = 1024 * 1024 * 16

	defaultCgroupDriver = "cgroupfs"
)

// CRIService includes all methods necessary for a CRI backend.
type CRIService interface {
	runtimeapi.RuntimeServiceServer
	runtimeapi.ImageServiceServer
}

type serviceCommon interface {
	Start() error
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

// DockerService is an interface that embeds the new RuntimeService and
// ImageService interfaces.
type DockerService interface {
	CRIService
	serviceCommon
}

var internalLabelKeys = []string{containerTypeLabelKey, containerLogPathLabelKey, sandboxIDLabelKey}

// NewDockerService creates a new `DockerService`
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
		containerManager:      containermanager.NewContainerManager(cgroupsName, client),
		checkpointManager:     checkpointManager,
		networkReady:          make(map[string]bool),
		containerCleanupInfos: make(map[string]*containerCleanupInfo),
		containerStatsCache:   newContainerStatsCache(),
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
	logrus.Infof("Hairpin mode is set to %s", pluginSettings.HairpinMode)

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
	logrus.Infof(
		"Docker cri networking managed by network plugin %s",
		plug.Name(),
	)

	dockerInfo, err := ds.getDockerInfo()
	if err != nil {
		return nil, fmt.Errorf("Failed to execute Info() call to the Docker client")
	}
	logrus.Debugf("Docker Info: %+v", dockerInfo)
	ds.dockerRootDir = dockerInfo.DockerRootDir

	// skipping cgroup driver checks for Windows
	if runtime.GOOS == "linux" {
		// NOTE: cgroup driver is only detectable in docker 1.11+
		cgroupDriver := defaultCgroupDriver
		if len(dockerInfo.CgroupDriver) == 0 {
			logrus.Info("No cgroup driver is set in Docker")
			logrus.Infof("Falling back to use the default driver %s", cgroupDriver)
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
		logrus.Infof("Setting cgroupDriver %s", cgroupDriver)
		ds.cgroupDriver = cgroupDriver
	}

	// Register prometheus metrics.
	metrics.Register()

	go ds.startStatsCollection()

	return ds, nil
}

type dockerService struct {
	// This handles unimplemented methods unless cri-dockerd overrides them
	runtimeapi.UnimplementedRuntimeServiceServer

	client           libdocker.DockerClientInterface
	os               config.OSInterface
	podSandboxImage  string
	streamingRuntime *streaming.StreamingRuntime
	streamingServer  streaming.Server

	network *network.PluginManager
	// Map of podSandboxID :: network-is-ready
	networkReady     map[string]bool
	networkReadyLock sync.Mutex

	containerManager containermanager.ContainerManager
	// cgroup driver used by Docker runtime.
	cgroupDriver      string
	checkpointManager store.CheckpointManager

	// cache for 'docker version' and 'docker info'
	systemInfoCache utils.Cache

	// docker root directory
	dockerRootDir string

	containerStatsCache *containerStatsCache

	// containerCleanupInfos maps container IDs to the `containerCleanupInfo` structs
	// needed to clean up after containers have been removed.
	// (see `applyPlatformSpecificDockerConfig` and `performPlatformSpecificContainerCleanup`
	// methods for more info).
	containerCleanupInfos map[string]*containerCleanupInfo
	cleanupInfosLock      sync.RWMutex

	// runtimeInfoLock sync.RWMutex
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
		RuntimeApiVersion: config.CRIVersion,
	}, nil
}

// getDockerVersion gets the version information from docker.
func (ds *dockerService) getDockerVersion() (*dockertypes.Version, error) {
	res, err := ds.systemInfoCache.Memoize("docker_version", systemInfoCacheMinTTL, func() (interface{}, error) {
		return ds.client.Version()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get docker version from dockerd: %v", err)
	}
	cachedValue := res.(*dockertypes.Version)
	// make a copy
	v := *cachedValue
	fixAPIVersion(&v)
	return &v, nil
}

// fixAPIVersion remedy Docker API version (e.g., 1.23) which is not semver compatible by
// adding a ".0" suffix
func fixAPIVersion(v *dockertypes.Version) {
	if v != nil {
		v.APIVersion = fmt.Sprintf("%s.0", v.APIVersion)
	}
}

// getDockerInfo gets the information of "docker info".
func (ds *dockerService) getDockerInfo() (*dockersystem.Info, error) {
	res, err := ds.systemInfoCache.Memoize("docker_info", systemInfoCacheMinTTL, func() (interface{}, error) {
		return ds.client.Info()
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get docker info from dockerd: %v", err)
	}
	info := res.(*dockersystem.Info)
	return info, nil
}

func (ds *dockerService) getRuntimeHandlers() ([]*runtimeapi.RuntimeHandler, error) {
	info, err := ds.getDockerInfo()
	if err != nil {
		return nil, err
	}
	handlersX, err := ds.systemInfoCache.Memoize("docker_info_handlers", systemInfoCacheMinTTL, func() (interface{}, error) {
		return getRuntimeHandlers(info)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get runtime handlers: %v", err)
	}
	return handlersX.([]*runtimeapi.RuntimeHandler), nil
}

func getRuntimeHandlers(info *dockersystem.Info) ([]*runtimeapi.RuntimeHandler, error) {
	var handlers []*runtimeapi.RuntimeHandler
	for dockerName, dockerRT := range info.Runtimes {
		var rro bool
		if kernelSupportsRRO {
			if ociFeaturesStr, ok := dockerRT.Status["org.opencontainers.runtime-spec.features"]; ok {
				// "org.opencontainers.runtime-spec.features" status is available since Docker v25 (API v1.44)
				var ociFeatures ociruntimefeatures.Features
				if err := json.Unmarshal([]byte(ociFeaturesStr), &ociFeatures); err != nil {
					return handlers, fmt.Errorf("failed to unmarshal %q: %v", ociFeaturesStr, err)
				}
				// "rro" mount type is supported since runc v1.1
				rro = slices.Contains(ociFeatures.MountOptions, "rro")
			}
		}
		features := &runtimeapi.RuntimeHandlerFeatures{
			RecursiveReadOnlyMounts: rro,
			UserNamespaces:          false, // TODO
		}
		handlers = append(handlers, &runtimeapi.RuntimeHandler{
			Name:     dockerName,
			Features: features,
		})
		if dockerName == info.DefaultRuntime {
			handlers = append([]*runtimeapi.RuntimeHandler{
				&runtimeapi.RuntimeHandler{
					Name:     "",
					Features: features,
				},
			}, handlers...)
		}
	}
	// info.Runtimes is unmarshalized as a map in Go, so we cannot preserve the original ordering
	slices.SortStableFunc(handlers, func(a, b *runtimeapi.RuntimeHandler) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return handlers, nil
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

	logrus.Infof("Docker cri received runtime config %+v", runtimeConfig)
	if ds.network != nil && runtimeConfig.NetworkConfig.PodCidr != "" {
		event := make(map[string]interface{})
		event[network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE_DETAIL_CIDR] = runtimeConfig.NetworkConfig.PodCidr
		ds.network.Event(network.NET_PLUGIN_EVENT_POD_CIDR_CHANGE, event)
	}

	return &runtimeapi.UpdateRuntimeConfigResponse{}, nil
}

// Start initializes and starts components in dockerService.
func (ds *dockerService) Start() error {
	ds.initCleanup()

	go func() {
		if err := ds.streamingServer.Start(true); err != nil {
			logrus.Errorf("Streaming backend stopped unexpectedly: %v", err)
			os.Exit(1)
		}
	}()

	return ds.containerManager.Start()
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
	if _, err := ds.getDockerVersion(); err != nil {
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
	handlers, err := ds.getRuntimeHandlers()
	if err != nil {
		return nil, err
	}
	resp := &runtimeapi.StatusResponse{
		Status:          status,
		RuntimeHandlers: handlers,
	}
	if r.Verbose {
		image := defaultSandboxImage
		podSandboxImage := ds.podSandboxImage
		if len(podSandboxImage) != 0 {
			image = podSandboxImage
		}
		config := map[string]interface{}{
			"sandboxImage": image,
		}
		configByt, err := json.Marshal(config)
		if err != nil {
			return nil, err
		}
		resp.Info = make(map[string]string)
		resp.Info["config"] = string(configByt)
	}
	return resp, nil
}

// RuntimeConfig returns the config of the runtime.
func (ds *dockerService) RuntimeConfig(
	_ context.Context,
	r *runtimeapi.RuntimeConfigRequest,
) (*runtimeapi.RuntimeConfigResponse, error) {
	resp := &runtimeapi.RuntimeConfigResponse{}
	if runtime.GOOS == "linux" {
		resp.Linux = &runtimeapi.LinuxRuntimeConfiguration{}
		switch ds.cgroupDriver {
		case "cgroupfs":
			resp.Linux.CgroupDriver = runtimeapi.CgroupDriver_CGROUPFS
		case "systemd":
			resp.Linux.CgroupDriver = runtimeapi.CgroupDriver_SYSTEMD
		default:
			return nil, fmt.Errorf("unknown cgroup driver: %s", ds.cgroupDriver)
		}
	}
	return resp, nil
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
	logrus.Debugf("Setting cgroup parent to (%s)", cgroupParent)
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

// initCleanup is responsible for cleaning up any crufts left by previous
// runs. If there are any errors, it simply logs them.
func (ds *dockerService) initCleanup() {
	errors := ds.platformSpecificContainerInitCleanup()

	for _, err := range errors {
		logrus.Errorf("Initialization error: %v", err)
	}
}

// getDockerAPIVersion gets the semver-compatible docker api version.
func (ds *dockerService) getDockerAPIVersion() (*semver.Version, error) {
	var dv *dockertypes.Version
	var err error

	dv, err = ds.getDockerVersion()
	if err != nil {
		return nil, err
	}

	apiVersion, err := semver.Parse(dv.APIVersion)
	if err != nil {
		return nil, err
	}
	return &apiVersion, nil
}
