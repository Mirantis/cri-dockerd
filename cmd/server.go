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

package cmd

import (
	"fmt"
	"runtime"

	"github.com/Mirantis/cri-dockerd/backend"
	"github.com/Mirantis/cri-dockerd/cmd/cri/options"
	"github.com/Mirantis/cri-dockerd/cmd/version"
	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/core"
	"github.com/Mirantis/cri-dockerd/streaming"
	"github.com/sirupsen/logrus"

	"net"
	"net/netip"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

const (
	componentDockerCRI = "cri-dockerd"
)

// NewDockerCRICommand creates a *cobra.Command object with default parameters
func NewDockerCRICommand(stopCh <-chan struct{}) *cobra.Command {
	cleanFlagSet := pflag.NewFlagSet(componentDockerCRI, pflag.ContinueOnError)
	kubeletFlags := options.NewDockerCRIFlags()

	cmd := &cobra.Command{
		Use:  componentDockerCRI,
		Long: `CRI that connects to the Docker Daemon`,
		// cri-dockerd has special flag parsing requirements to enforce flag precedence rules,
		// so we do all our parsing manually in Run, below.
		// DisableFlagParsing=true provides the full set of flags passed to cri-dockerd in the
		// `args` arg to Run, without Cobra's interference.
		DisableFlagParsing: true,
		Run: func(cmd *cobra.Command, args []string) {
			// initial flag parse, since we disable cobra's flag parsing
			if err := cleanFlagSet.Parse(args); err != nil {
				cmd.Usage()
				logrus.Fatal(err)
			}

			// check if there are non-flag arguments in the command line
			cmds := cleanFlagSet.Args()
			if len(cmds) > 0 {
				cmd.Usage()
				logrus.Fatalf("Unknown command: %s", cmds[0])
			}

			// short-circuit on help
			help, err := cleanFlagSet.GetBool("help")
			if err != nil {
				logrus.Fatal(`"help" flag is non-bool`)
			}
			if help {
				cmd.Help()
				return
			}

			verflag, _ := cleanFlagSet.GetBool("version")
			if verflag {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"%s %s\n",
					version.PlatformName,
					version.FullVersion(),
				)
				return
			}

			infoflag, _ := cleanFlagSet.GetBool("buildinfo")
			if infoflag {
				fmt.Fprintf(
					cmd.OutOrStdout(),
					"Program: %s\nVersion: %s\nGitCommit: %s\nGo version: %s\n",
					version.PlatformName,
					version.FullVersion(),
					version.GitCommit,
					runtime.Version(),
				)
				return
			}

			logFlag, _ := cleanFlagSet.GetString("log-level")
			if logFlag != "" {
				level, err := logrus.ParseLevel(logFlag)
				if err != nil {
					logrus.Fatalf("Unknown log level: %s", logFlag)
				}
				logrus.SetLevel(level)
			}

			if err := RunCriDockerd(kubeletFlags, stopCh); err != nil {
				logrus.Fatal(err)
			}
		},
	}

	// keep cleanFlagSet separate, so Cobra doesn't pollute it with the global flags
	kubeletFlags.AddFlags(cleanFlagSet)
	cleanFlagSet.BoolP("help", "h", false, fmt.Sprintf("Help for %s", cmd.Name()))
	cleanFlagSet.Bool("version", false, "Prints the version of cri-dockerd")
	cleanFlagSet.Bool("buildinfo", false, "Prints the build information about cri-dockerd")
	cleanFlagSet.String("log-level", "info", "The log level for cri-docker")

	// ugly, but necessary, because Cobra's default UsageFunc and HelpFunc pollute the flagset with global flags
	const usageFmt = "Usage:\n  %s\n\nFlags:\n%s"
	cmd.SetUsageFunc(func(cmd *cobra.Command) error {
		fmt.Fprintf(cmd.OutOrStderr(), usageFmt, cmd.UseLine(), cleanFlagSet.FlagUsagesWrapped(2))
		return nil
	})
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(
			cmd.OutOrStdout(),
			"%s\n\n"+usageFmt,
			cmd.Long,
			cmd.UseLine(),
			cleanFlagSet.FlagUsagesWrapped(2),
		)
	})
	return cmd
}

// RunCriDockerd starts cri-dockerd
func RunCriDockerd(f *options.DockerCRIFlags, stopCh <-chan struct{}) error {
	logrus.Infof("Starting %s %s", version.PlatformName, version.FullVersion())
	r := &f.ContainerRuntimeOptions

	// Initialize docker client configuration.
	dockerClientConfig := &config.ClientConfig{
		DockerEndpoint:            r.DockerEndpoint,
		RuntimeRequestTimeout:     r.RuntimeRequestTimeout.Duration,
		ImagePullProgressDeadline: r.ImagePullProgressDeadline.Duration,
	}

	// Initialize network plugin settings.
	pluginSettings := config.NetworkPluginSettings{
		HairpinMode:        config.HairpinModeVar.Mode(),
		PluginName:         f.NetworkPluginName,
		PluginConfDir:      f.CNIConfDir,
		PluginBinDirString: f.CNIBinDir,
		PluginCacheDir:     f.CNICacheDir,
		MTU:                int(f.NetworkPluginMTU),
		NonMasqueradeCIDR:  f.NonMasqueradeCIDR,
	}

	config.IPv6DualStackEnabled = f.IPv6DualStackEnabled

	var resolvedAddr string
	if r.StreamingBindAddr != "" {
		// See whether a port was specified as part of the declaration
		addrPort, err := netip.ParseAddrPort(r.StreamingBindAddr)
		if err != nil {
			addr, err := netip.ParseAddr(r.StreamingBindAddr)
			if err != nil {
				logrus.Fatalf("Could not parse the given streaming bind address: %s", r.StreamingBindAddr)
			}
			resolvedAddr = net.JoinHostPort(addr.String(), "0")
		} else {
			resolvedAddr = addrPort.String()
		}
	}

	// Initialize streaming configuration. (Not using TLS now)
	streamingConfig := &streaming.Config{
		// Use a relative redirect (no scheme or host).
		BaseURL:                         &url.URL{Path: "/cri/"},
		Addr:                            resolvedAddr,
		StreamIdleTimeout:               r.StreamingConnectionIdleTimeout.Duration,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
	}

	// Standalone cri-dockerd will always start the local streaming backend.
	ds, err := core.NewDockerService(
		dockerClientConfig,
		r.PodSandboxImage,
		streamingConfig,
		&pluginSettings,
		f.RuntimeCgroups,
		r.CgroupDriver,
		r.CriDockerdRootDirectory,
	)
	if err != nil {
		return err
	}

	if _, err := ds.UpdateRuntimeConfig(nil, &runtimeapi.UpdateRuntimeConfigRequest{
		RuntimeConfig: &runtimeapi.RuntimeConfig{
			NetworkConfig: &runtimeapi.NetworkConfig{
				PodCidr: f.PodCIDR,
			},
		},
	}); err != nil {
		return err
	}

	logrus.Info("Starting the GRPC backend for the Docker CRI interface.")
	server := backend.NewCriDockerServer(f.RemoteRuntimeEndpoint, ds)
	if err := server.Start(); err != nil {
		return err
	}

	<-stopCh
	return nil
}
