---
weight: 2
---

Integration tests can be ran on your local machine without the need for minikube.

## Prerequisites

Before running the tests, you will need the following installed on your local
machine

### cri-tools

Install [`cri-tools`](https://github.com/kubernetes-sigs/cri-tools) using the
project's instructions. `critest` will need to be available in your system's
PATH in order for the make command to work.

> This will install the specific version of cri-tools. CI will always pull the
> latest version and test against it. If your changes pass locally but fail
> CI, you may need to update your local version of `cri-tools`.

### Packages

A couple of system packages will also be needed. To install them on an apt based
system, run the following command.

``` bash
apt install nsenter
```

## Start cri-dockerd

There is a command to start an instance of `cri-dockerd`. This will ask for
`sudo` permissions before it starts running. This process will need to be left
running in its own terminal.

``` bash
make run
```

## Run the tests

Open a second terminal and run the tests.

``` bash
make integration
```

The tests will run and the results will be printed to the terminal with a summary
at the end of the test run.
