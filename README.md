# cri-dockerd

This adapter provides a shim for [Docker Engine](https://docs.docker.com/engine/)
that lets you control Docker via the
Kubernetes [Container Runtime Interface](https://github.com/kubernetes/cri-api#readme).

## Motivation

Mirantis and Docker have agreed to partner to maintain the shim code standalone outside Kubernetes, as a conformant CRI
interface for the Docker Engine API. For Mirantis customers, that means that Docker Engine’s commercially supported
version, Mirantis Container Runtime (MCR), will be CRI compliant. This means that you can continue to build Kubernetes
based on the Docker Engine as before, just switching from the built in dockershim to the external one.

Mirantis and Docker intend to work together on making sure it continues to work as well as before and that it
passes all the conformance tests and continues to work just like the built in version did. Mirantis will be using
this in Mirantis Kubernetes Engine, and Docker will continue to ship this shim in Docker Desktop.

You can find more information about the context for this
tool in [Don't Panic: Kubernetes and Docker](https://blog.k8s.io/2020/12/02/dont-panic-kubernetes-and-docker/)
and on the Mirantis
[blog](https://www.mirantis.com/blog/mirantis-to-take-over-support-of-kubernetes-dockershim-2/).

## Build and install

To build this code (in a POSIX environment):
```shell
mkdir bin
VERSION=$((git describe --abbrev=0 --tags | sed -e 's/v//') || echo $(cat VERSION)-$(git log -1 --pretty='%h')) PRERELEASE=$(grep -q dev <<< "${VERSION}" && echo "pre" || echo "") REVISION=$(git log -1 --pretty='%h')
export CRI_DOCKERD_LDFLAGS=-ldflags "-X github.com/Mirantis/cri-dockerd/version.Version=${VERSION} -X github.com/Mirantis/cri-dockerd/version.PreRelease=${PRERELEASE} -X github.com/Mirantis/cri-dockerd/version.BuildTime=${BUILD_DATE} -X github.com/Mirantis/cri-dockerd/version.GitCommit=${REVISION}"
go get && go build ${CRI_DOCKERD_LDFLAGS} -o ../bin/cri-dockerd
```

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH` is a known build target for golang

To install, on a Linux system that uses systemd, and already has Docker Engine installed
```shell
# Run these commands as root
mkdir -p /usr/local/bin
install -o root -g root -m 0755 bin/cri-dockerd /usr/local/bin/cri-dockerd
cp -a packaging/systemd/* /etc/systemd/system
sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
systemctl daemon-reload
systemctl enable cri-docker.service
systemctl enable --now cri-docker.socket
```
