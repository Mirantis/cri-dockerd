# cri-dockerd

This adapter provides a shim for [Docker Engine](https://docs.docker.com/engine/)
that lets you control Docker via the
Kubernetes [Container Runtime Interface](https://github.com/kubernetes/cri-api#readme).

## IMPORTANT

For users running `0.2.5` or above, the default network plugin is `cni`. Kubernetes 1.24+ has removed `kubenet` and
other network plumbing from upstream as part of the `dockershim` removal/deprecation. In order for a cluster to become
operational, Calico, Flannel, Weave, or another CNI should be used.

For CI workflows, basic functionality can be provided via [`containernetworking/plugins`](
https://github.com/containernetworking/plugins).

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

To begin following the build process for this code, clone this repository in your local environment:

## To use with Kubernetes

The default network plugin for `cri-dockerd` is set to `cni` on Linux. To change this, `--network-plugin=${plugin}`
can be passed in as a command line argument if invoked manually, or the systemd unit file
(`/usr/lib/systemd/system/cri-docker.service` if not enabled yet,
or `/etc/systemd/system/multi-user.target.wants/cri-docker.service` as a symlink if it is enabled) should be
edited to add this argument, followed by `systemctl daemon-reload` and restarting the service (if running)

```shell
git clone https://github.com/Mirantis/cri-dockerd.git
```

The above step creates a local directory called ```cri-dockerd``` which you will need for the following steps.

To build this code (in a POSIX environment):

```shell
mkdir bin
VERSION=$((git describe --abbrev=0 --tags | sed -e 's/v//') || echo $(cat VERSION)-$(git log -1 --pretty='%h')) PRERELEASE=$(grep -q dev <<< "${VERSION}" && echo "pre" || echo "") REVISION=$(git log -1 --pretty='%h')
go build -ldflags="-X github.com/Mirantis/cri-dockerd/version.Version='$VERSION}' -X github.com/Mirantis/cri-dockerd/version.PreRelease='$PRERELEASE' -X github.com/Mirantis/cri-dockerd/version.BuildTime='$BUILD_DATE' -X github.com/Mirantis/cri-dockerd/version.GitCommit='$REVISION'" -o cri-dockerd
```

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH` is a known build target for golang

To install, on a Linux system that uses systemd, and already has Docker Engine installed

```shell
# Run these commands as root
###Install GO###
wget https://storage.googleapis.com/golang/getgo/installer_linux
chmod +x ./installer_linux
./installer_linux
source ~/.bash_profile

cd cri-dockerd
mkdir bin
go build -o bin/cri-dockerd
mkdir -p /usr/local/bin
install -o root -g root -m 0755 bin/cri-dockerd /usr/local/bin/cri-dockerd
cp -a packaging/systemd/* /etc/systemd/system
sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
systemctl daemon-reload
systemctl enable cri-docker.service
systemctl enable --now cri-docker.socket
```
