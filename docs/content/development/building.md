---
weight: 1
---

If you would like to build the project yourself, you will need to have Go installed.
You can find directions for installing the latest version on its website:

[Install the latest version of Go](https://golang.org/doc/install)

Once you have Go installed, you can build the project by running the following command:

```shell
make cri-dockerd
```

This will output the binary to the project's root directory as `cri-dockerd`.
You can then run it directly or install it using the manual process above.

To build for a specific architecture, add `ARCH=` as an argument, where `ARCH`
is a known build target for Go.
