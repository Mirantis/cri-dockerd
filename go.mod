module github.com/Mirantis/cri-dockerd

go 1.21

require (
	github.com/Microsoft/hcsshim v0.8.25
	github.com/armon/circbuf v0.0.0-20150827004946-bbbad097214e
	github.com/blang/semver v3.5.1+incompatible
	github.com/containernetworking/cni v1.1.2
	github.com/coreos/go-systemd/v22 v22.5.0
	github.com/davecgh/go-spew v1.1.1
	github.com/docker/distribution v2.8.3+incompatible
	github.com/docker/docker v24.0.7+incompatible
	github.com/docker/go-connections v0.4.0
	github.com/emicklei/go-restful v2.16.0+incompatible
	github.com/golang/mock v1.6.0
	github.com/opencontainers/go-digest v1.0.0
	github.com/opencontainers/image-spec v1.1.0-rc5
	github.com/opencontainers/runc v1.1.10
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/sync v0.5.0
	golang.org/x/sys v0.14.0
	google.golang.org/grpc v1.59.0
	k8s.io/api v0.27.8
	k8s.io/apimachinery v0.27.8
	k8s.io/apiserver v0.27.8
	k8s.io/client-go v0.27.8
	k8s.io/component-base v0.27.8
	k8s.io/component-helpers v0.27.8
	k8s.io/cri-api v0.27.8
	k8s.io/cri-api/v1alpha2 v0.25.8
	k8s.io/kubernetes v1.27.8
	k8s.io/utils v0.0.0-20230209194617-a36077c30491
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/JeffAshton/win_pdh v0.0.0-20161109143554-76bb4ee9f0ab // indirect
	github.com/Microsoft/go-winio v0.4.17 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/antlr/antlr4/runtime/Go/antlr v1.4.10 // indirect
	github.com/asaskevich/govalidator v0.0.0-20190424111038-f61b66f89f4a // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/containerd/cgroups v1.0.1 // indirect
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/btree v1.1.2 // indirect
	github.com/google/cadvisor v0.47.2 // indirect
	github.com/google/cel-go v0.12.7 // indirect
	github.com/google/gnostic v0.7.0 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.1.0 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jonboulle/clockwork v0.3.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/mapstructure v1.4.1 // indirect
	github.com/moby/spdystream v0.2.0 // indirect
	github.com/moby/sys/mountinfo v0.6.2 // indirect
	github.com/moby/term v0.0.0-20221205130635-1aeaba878587 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20220909204839-494a5a6aca78 // indirect
	github.com/opencontainers/selinux v1.10.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/stoewer/go-strcase v1.2.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/vishvananda/netns v0.0.2 // indirect
	github.com/xiang90/probing v0.0.0-20221125231312-a49e3df8f510 // indirect
	go.etcd.io/etcd/api/v3 v3.5.7 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.7 // indirect
	go.etcd.io/etcd/client/v3 v3.5.7 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.46.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/otel/sdk v1.21.0 // indirect
	go.opentelemetry.io/otel/trace v1.21.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.9.0 // indirect
	go.uber.org/zap v1.24.0 // indirect
	golang.org/x/crypto v0.14.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.11.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/cloud-provider v0.27.8 // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230501164219-8b0f38b5fd1f // indirect
	k8s.io/mount-utils v0.27.8 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.1.2 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	google.golang.org/grpc/naming => google.golang.org/grpc v1.29.0
	k8s.io/api => k8s.io/api v0.27.8
	k8s.io/apimachinery => k8s.io/apimachinery v0.27.8
	k8s.io/apiserver => k8s.io/apiserver v0.27.8
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.27.8
	k8s.io/client-go => k8s.io/client-go v0.27.8
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.27.8
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.27.8
	k8s.io/code-generator => k8s.io/code-generator v0.27.8
	k8s.io/component-base => k8s.io/component-base v0.27.8
	k8s.io/component-helpers => k8s.io/component-helpers v0.26.0
	k8s.io/controller-manager => k8s.io/controller-manager v0.27.8
	k8s.io/cri-api => k8s.io/cri-api v0.27.8
	k8s.io/cri-api/v1alpha2 => k8s.io/cri-api v0.25.16
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.27.8
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.27.8
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.27.8
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.27.8
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.27.8
	k8s.io/kubectl => k8s.io/kubectl v0.27.8
	k8s.io/kubelet => k8s.io/kubelet v0.27.8
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.27.8
	k8s.io/metrics => k8s.io/metrics v0.27.8
	k8s.io/mount-utils => k8s.io/mount-utils v0.27.8
	k8s.io/pod-security-admission => k8s.io/pod-security-admission v0.27.8
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.27.8
)
