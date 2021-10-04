/*
Copyright 2015 The Kubernetes Authors.

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

package app

import (
	"fmt"
	"github.com/Mirantis/cri-dockerd/core"
	"github.com/Mirantis/cri-dockerd/pkg/app/options"
	dockerremote "github.com/Mirantis/cri-dockerd/remote"
	"github.com/Mirantis/cri-dockerd/version"
	"github.com/sirupsen/logrus"

	"k8s.io/component-base/cli/flag"
	utilflag "k8s.io/component-base/cli/flag"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/cri/streaming"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	componentDockerCRI = "cri-dockerd"
)

// NewDockerCRICommand creates a *cobra.Command object with default parameters
func NewDockerCRICommand(stopCh <-chan struct{}) *cobra.Command {
	cleanFlagSet := pflag.NewFlagSet(componentDockerCRI, pflag.ContinueOnError)
	cleanFlagSet.SetNormalizeFunc(flag.WordSepNormalizeFunc)
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
					cmd.OutOrStderr(),
					"%s %s\n",
					version.PlatformName,
					version.FullVersion(),
				)
				return
			}

			// short-circuit on verflag
			utilflag.PrintFlags(cleanFlagSet)

			if err := RunCriDockerd(kubeletFlags, stopCh); err != nil {
				logrus.Fatal(err)
			}
		},
	}

	// keep cleanFlagSet separate, so Cobra doesn't pollute it with the global flags
	kubeletFlags.AddFlags(cleanFlagSet)
	options.AddGlobalFlags(cleanFlagSet)
	cleanFlagSet.BoolP("help", "h", false, fmt.Sprintf("Help for %s", cmd.Name()))
	cleanFlagSet.Bool("version", false, "Prints the version of cri-dockerd")

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
	r := &f.ContainerRuntimeOptions

	// Initialize docker client configuration.
	dockerClientConfig := &core.ClientConfig{
		DockerEndpoint:            r.DockerEndpoint,
		RuntimeRequestTimeout:     r.RuntimeRequestTimeout.Duration,
		ImagePullProgressDeadline: r.ImagePullProgressDeadline.Duration,
	}

	// Initialize network plugin settings.
	pluginSettings := core.NetworkPluginSettings{
		HairpinMode:        "none",
		PluginName:         f.NetworkPluginName,
		PluginConfDir:      f.CNIConfDir,
		PluginBinDirString: f.CNIBinDir,
		PluginCacheDir:     f.CNICacheDir,
		MTU:                int(f.NetworkPluginMTU),
		NonMasqueradeCIDR:  f.NonMasqueradeCIDR,
	}

	// Initialize streaming configuration. (Not using TLS now)
	streamingConfig := &streaming.Config{
		// Use a relative redirect (no scheme or host).
		BaseURL:                         &url.URL{Path: "/cri/"},
		StreamIdleTimeout:               r.StreamingConnectionIdleTimeout.Duration,
		StreamCreationTimeout:           streaming.DefaultConfig.StreamCreationTimeout,
		SupportedRemoteCommandProtocols: streaming.DefaultConfig.SupportedRemoteCommandProtocols,
		SupportedPortForwardProtocols:   streaming.DefaultConfig.SupportedPortForwardProtocols,
	}

	// Standalone cri-dockerd will always start the local streaming server.
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

	logrus.Infof("Starting the GRPC server for the Docker CRI interface.")
	server := dockerremote.NewDockerServer(f.RemoteRuntimeEndpoint, ds)
	if err := server.Start(); err != nil {
		return err
	}

	<-stopCh
	return nil
}
