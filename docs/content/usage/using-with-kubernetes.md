---
weight: 3
---

The default network plugin for `cri-dockerd` is set to `cni` on Linux. There are
a few ways to change this depending on how you are running the binary.

`--network-plugin=${plugin}` can be passed in as a command line argument when
 - running the binary directly
 - adding to `/usr/lib/systemd/system/cri-docker.service` if a service isn't enabled
 - adding to `/etc/systemd/system/multi-user.target.wants/cri-docker.service` if a service is enabled

Run `systemctl daemon-reload` to restart the service if it was already running.
