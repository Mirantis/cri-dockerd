<p align="center">
  <img src="docs/static/images/logo.svg" alt="docker and kubernetes interact"/>
</p>

# cri-dockerd

This adapter provides a shim for [Docker Engine](https://docs.docker.com/engine/)
that lets you control Docker via the
Kubernetes [Container Runtime Interface](https://github.com/kubernetes/cri-api#readme).

Take a look at the [official docs](https://mirantis.github.io/cri-dockerd/) for more information.

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

## Community

We can be found on the [Kubernetes Slack](https://communityinviter.com/apps/kubernetes/community) in the [`#cri-dockerd`](https://kubernetes.slack.com/messages/cri-dockerd) channel.

## Using cri-dockerd

### Install

Refer to the [install page](https://mirantis.github.io/cri-dockerd/usage/install/) for instructions on how to install `cri-dockerd` using a package manager.

## Advanced Setup

### Installing manually

If you want to run `cri-dockerd` on an unsupported platform, instructions can be found on the [manual install page](https://mirantis.github.io/cri-dockerd/usage/install-manually/).

### To use with Kubernetes

If you want to use `cri-dockerd` with Kubernetes, you can find instructions on the [Kubernetes page](https://mirantis.github.io/cri-dockerd/usage/using-with-kubernetes/).

## Developing cri-dockerd

We welcome contributions to `cri-dockerd`. If you would like to contribute, please refer to the development section of the [official docs](https://mirantis.github.io/cri-dockerd/development/).

## Documentation

The docs are generated using [Hugo](https://gohugo.io/) and the [Geekdocs](https://themes.gohugo.io/hugo-geekdoc/) theme. Hugo will need to be installed to generate the docs found in the `docs/` directory.

### Editing Docs

The docs can be ran locally with hot-reloading to make editing easier. To do so, run the following command in the project's root directory:

```bash
make docs
```

This will launch the development server that is included with Hugo. You can then access the docs at http://localhost:1313/
