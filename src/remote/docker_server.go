// +build !dockerless

/*
Copyright 2016 The Kubernetes Authors.

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

package remote

import (
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/Mirantis/cri-dockerd/core"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/kubelet/util"
)

// maxMsgSize use 16MB as the default message size limit.
// grpc library default is 4MB
const maxMsgSize = 1024 * 1024 * 16

// DockerServer is the grpc server of cri-dockerd.
type DockerServer struct {
	// endpoint is the endpoint to serve on.
	endpoint string
	// service is the docker service which implements runtime and image services.
	service core.CRIService
	// server is the grpc server.
	server *grpc.Server
}

// NewDockerServer creates the cri-dockerd grpc server.
func NewDockerServer(endpoint string, s core.CRIService) *DockerServer {
	return &DockerServer{
		endpoint: endpoint,
		service:  s,
	}
}

func getListener(addr string) (net.Listener, error) {
	addrSlice := strings.SplitN(addr, "://", 2)
	proto := addrSlice[0]
	listenAddr := addrSlice[1]
	switch proto {
	case "fd":
		return listenFD(listenAddr)
	default:
		return util.CreateListener(addr)
	}
}

// Start starts the cri-dockerd grpc server.
func (s *DockerServer) Start() error {
	// Start the internal service.
	if err := s.service.Start(); err != nil {
		klog.ErrorS(err, "Unable to start cri-dockerd service")
		return err
	}

	klog.V(2).InfoS("Start cri-dockerd grpc server")
	l, err := getListener(s.endpoint)
	if err != nil {
		return fmt.Errorf("cri-dockerd failed to listen on %q: %v", s.endpoint, err)
	}
	// Create the grpc server and register runtime and image services.
	s.server = grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMsgSize),
		grpc.MaxSendMsgSize(maxMsgSize),
	)
	runtimeapi.RegisterRuntimeServiceServer(s.server, s.service)
	runtimeapi.RegisterImageServiceServer(s.server, s.service)
	go func() {
		if err := s.server.Serve(l); err != nil {
			klog.ErrorS(err, "Failed to serve connections from cri-dockerd")
			os.Exit(1)
		}
	}()
	handleNotify()
	return nil
}