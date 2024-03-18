---
weight: 1
---

### Prerequisites

If you want to build `cri-dockerd` from source, you will need the following:

- [Install the latest version of Go](https://golang.org/doc/install)

### Building

To build the project yourself, you will need a local copy of the repository. You can clone the repository by running the following command:

```shell
git clone https://github.com/Mirantis/cri-dockerd.git
```

The above step creates a local directory called `cri-dockerd` which you will need for the following steps.

You can build the project by running the following command from within the `cri-dockerd` directory:

```shell
make cri-dockerd
```

This will output the binary to the project's root directory as `cri-dockerd`. You can then run it directly or install it using [the manual install process](/usage/manual-install/).

> Note: With a local copy of the repo, the service files can be installed from the `packaging/systemd` directory.

### Building for a Specific Architecture

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH`
is a [known build target for Go](https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63#goarch-values).

```shell
ARCH=amd64 make cri-dockerd
```
