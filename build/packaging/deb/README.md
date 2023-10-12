# Building your own CRI Docker deb package

`.deb` packages can be built from this directory with the following syntax

```shell
make deb
```

Artifacts will be located in `debbuild` under the following directory structure:
`debbuild/$distro-$distro_version/`

## Specifying a specific distro

```shell
make ubuntu
```

## Specifying a specific distro version
```shell
make ubuntu-xenial
```