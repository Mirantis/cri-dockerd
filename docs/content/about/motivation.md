---
---

Mirantis and Docker have agreed to partner to maintain the shim code standalone outside Kubernetes, as a conformant CRI
interface for the Docker Engine API. This means that you can continue to build Kubernetes
based on the Docker Engine as before, just switching from the built in dockershim to the external one.

Mirantis and Docker intend to work together on making sure it continues to work as well as before and that it
passes all the conformance tests and continues to work just like the built in version did. Mirantis will be using
this in Mirantis Kubernetes Engine, and Docker will continue to ship this shim in Docker Desktop. For Mirantis customers,
this means that Docker Engine’s commercially supported version, Mirantis Container Runtime (MCR), will be CRI compliant.

You can find more information about the context for this
tool in [Don't Panic: Kubernetes and Docker](https://blog.k8s.io/2020/12/02/dont-panic-kubernetes-and-docker/)
and on the Mirantis
[blog](https://www.mirantis.com/blog/mirantis-to-take-over-support-of-kubernetes-dockershim-2/).
