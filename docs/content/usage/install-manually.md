---
weight: 2
---

If you would like to install the project manually, you will need to place the binary
somewhere in your `PATH` and setup a service to run it. The following commands is
a manual install for a Linux system using systemd:

```shell
install -o root -g root -m 0755 cri-dockerd /usr/local/bin/cri-dockerd
install packaging/systemd/* /etc/systemd/system
sed -i -e 's,/usr/bin/cri-dockerd,/usr/local/bin/cri-dockerd,' /etc/systemd/system/cri-docker.service
systemctl daemon-reload
systemctl enable --now cri-docker.socket
```
