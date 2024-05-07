---
weight: 2
---

If you want to run `cri-dockerd` on an unsupported platform, you can manually install the binary and setup the service. This is a more involved process and requires you to manually install the binary and setup the service.
The following instructions are for a Linux system using systemd.

> Note: These commands should be ran as root

> Note: the release packages will install to /usr/bin which is reserved for binaries managed by a package manager. Manual installation doesn't involve a package manager and thus uses /usr/local/bin and the service file must be edited to reflect this.

#### Download

Begin by downloading the latest `.tgz` archive for your platform from the [releases page](https://github.com/Mirantis/cri-dockerd/releases/latest).

Next, extract the archive and change to the extracted directory:

```shell
tar -xzf cri-dockerd-<version>.linux-amd64.tgz
```

#### Install

Install the binary to `/usr/local/bin` by running the following command as root from within the extracted directory:

```shell
install -o root -g root -m 0755 cri-dockerd /usr/local/bin/cri-dockerd
```

> Note: the `cp` command cannot be used instead of `install`.

#### Setup the systemd service

Setup the systemd service and socket to start `cri-dockerd`. Download the systemd service and socket files from the `packaging/systemd` directory.

```shell
wget https://raw.githubusercontent.com/Mirantis/cri-dockerd/master/packaging/systemd/cri-docker.service
wget https://raw.githubusercontent.com/Mirantis/cri-dockerd/master/packaging/systemd/cri-docker.socket
```

```shell
install cri-docker.service /etc/systemd/system
install cri-docker.socket /etc/systemd/system
```

Enable and start the `cri-dockerd` service:

```shell
sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
systemctl daemon-reload
systemctl enable --now cri-docker.socket
```

Verify that `cri-dockerd` is installed and running:

```shell
systemctl status cri-docker
```
