---
weight: 1
---

The easiest way to install `cri-dockerd` is to use one of the pre-built packages from the [releases page](https://github.com/Mirantis/cri-dockerd/releases). There are numerous supported platforms and using a pre-built package will install the binary and setup your system to run it as a service.

Pre-built `.tgz` archives are also available for [manual installation](/usage/manual-install). This is useful if you want to run `cri-dockerd` on an unsupported platform. This is a more involved process and requires you to manually install the binary and setup the service.

### Prerequisites

Before installing `cri-dockerd`, ensure that you have the following prerequisites installed and running on your system:

- [docker](https://docs.docker.com/engine/install/)

### Ubuntu/Debian

`.deb` packages are available for Ubuntu and Debian systems. They make it easy to install `cri-dockerd` using `apt-get` or `dpkg` commands.

Begin by downloading the latest `.deb` package for your distro's version from the [releases page](https://github.com/Mirantis/cri-dockerd/releases/latest).

Next, install the package using your choice of package manager:

```bash
apt-get install ./cri-dockerd-<version>.deb
```

or

```bash
dpkg -i cri-dockerd-<version>.deb
```

Verify that `cri-dockerd` is installed and running:

```bash
systemctl status cri-docker
```

The services should be `active (running)` and `enabled` to start on boot.

### Fedora/CentOS/RHEL

`.rpm` packages are available for Fedora, CentOS, and RHEL systems. They make it easy to install `cri-dockerd` using `dnf` or `yum` commands.

Begin by downloading the latest `.rpm` package for your distro's version from the [releases page](https://github.com/Mirantis/cri-dockerd/releases/latest).

Next, install the package using your choice of package manager:

```bash
dnf install cri-dockerd-<version>.rpm
```

or

```bash
yum install cri-dockerd-<version>.rpm
```

Enable and start the `cri-dockerd` service:

```bash
systemctl enable cri-docker && systemctl start cri-docker
```

Verify that `cri-dockerd` is installed and running:

```bash
systemctl status cri-docker
```

The services should be `active (running)` and `enabled` to start on boot.
