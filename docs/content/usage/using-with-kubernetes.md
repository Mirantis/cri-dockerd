---
weight: 3
---

The default network plugin for `cri-dockerd` is set to `cni` on Linux. To change this, `--network-plugin=${plugin}`
can be passed in as a command line argument if invoked manually, or the systemd unit file
(`/usr/lib/systemd/system/cri-docker.service` if not enabled yet,
or `/etc/systemd/system/multi-user.target.wants/cri-docker.service` as a symlink if it is enabled) should be
edited to add this argument, followed by `systemctl daemon-reload` and restarting the service (if running)
