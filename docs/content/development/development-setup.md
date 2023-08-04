---
weight: 2
---

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
