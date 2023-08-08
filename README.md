![docker and kubernetes interact](docs/static/logo.svg)

# cri-dockerd

This adapter provides a shim for [Docker Engine](https://docs.docker.com/engine/)
that lets you control Docker via the
Kubernetes [Container Runtime Interface](https://github.com/kubernetes/cri-api#readme).

## IMPORTANT

For users running `0.2.5` or above, the default network plugin is `cni`. Kubernetes 1.24+ has removed `kubenet` and
other network plumbing from upstream as part of the `dockershim` removal/deprecation. In order for a cluster to become
operational, Calico, Flannel, Weave, or another CNI should be used.

For CI workflows, basic functionality can be provided via [`containernetworking/plugins`](https://github.com/containernetworking/plugins).

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

## Using cri-dockerd

### Install

The easiest way to install `cri-dockerd` is to use one of the pre-built binaries or
packages from the [releases page](https://github.com/Mirantis/cri-dockerd/releases).
There are numerous supported platforms and using a pre-built package will install
the binary and setup your system to run it as a service.

Please refer to your platform's documentation for how to install a package for
additional help with these.

## Advanced Setup

### Installing manually

> Note: the release packages will install to /usr/bin which is reserved for
> binaries managed by a package manager. Manual installation doesn't involve a
> package manager and thus uses /usr/local/bin and the service file must be edited
> to reflect this.

If you would like to install the project manually, you will need to place the binary
somewhere in your `PATH` and setup a service to run it. The following command is
a manual install for a Linux system using systemd:

```shell
git clone https://github.com/Mirantis/cri-dockerd.git
```

The above step creates a local directory called `cri-dockerd` which you will need for the following steps.

To build this code (in a POSIX environment):

<https://go.dev/doc/install>

```shell
cd cri-dockerd
make cri-dockerd
```

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH` is a known build target for golang

You can find pre-compiled binaries and deb/rpm packages under:

<https://github.com/Mirantis/cri-dockerd/releases>

Where `VERSION` is the latest available cri-dockerd version:

`https://github.com/Mirantis/cri-dockerd/releases/download/v${VERSION}/cri-dockerd-${VERSION}.${ARCH}.tgz`

To install, on a Linux system that uses systemd, and already has Docker Engine installed

```shell
# Run these commands as root

cd cri-dockerd
mkdir -p /usr/local/bin
install -o root -g root -m 0755 cri-dockerd /usr/local/bin/cri-dockerd
install packaging/systemd/* /etc/systemd/system
sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
systemctl daemon-reload
systemctl enable --now cri-docker.socket
```

### To use with Kubernetes

The default network plugin for `cri-dockerd` is set to `cni` on Linux. There are
a few ways to change this depending on how you are running the binary.

`--network-plugin=${plugin}` can be passed in as a command line argument when
 - running the binary directly
 - adding to `/usr/lib/systemd/system/cri-docker.service` if a service isn't enabled
 - adding to `/etc/systemd/system/multi-user.target.wants/cri-docker.service` if a service is enabled

Run `systemctl daemon-reload` to restart the service if it was already running.

## Development

### Building

If you would like to build the project yourself, you will need to have Go installed.
You can find directions for installing the latest version on its website:

[Install the latest version of Go](https://golang.org/doc/install)

Once you have Go installed, you can build the project by running the following command:

```shell
make cri-dockerd
```

This will output the binary to the project's root directory as `cri-dockerd`.
You can then run it directly or install it using the manual process above.

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH`
is a known build target for Go.

```shell
ARCH=amd64 make cri-dockerd
```

### Development Setup

When developing, it is nice to have a separate environment to test in so that
you don't have to worry about breaking your system. An easy way to do this is
by setting up a minikube cluster since it uses `cri-dockerd` by default. Follow
the [minikube installation instructions](https://minikube.sigs.k8s.io/docs/start/)
to get it installed.

You'll then be able to create a cluster in minikube's VM by running:

```shell
minikube start
```

Once the cluster is up, we have a `make` command that will build `cri-dockerd`
and swap it out for the version running in the cluster. You can run this command
by running:

```shell
make dev
```

## Docs

This folder contains the files used to generate the `cri-dockerd` documentation.

The docs are generated using [Hugo](https://gohugo.io/) and the [Geekdocs](https://themes.gohugo.io/hugo-geekdoc/) theme.

### Editing Docs

The docs can be ran locally with hot-reloading to make editing easier. To do so,
run the following command in the project's root directory:

```bash
make docs
```

This will launch the development server that is included with Hugo. You can then
access the docs at http://localhost:1313/
