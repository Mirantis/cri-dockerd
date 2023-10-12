# Building your own Docker rpm package

`.rpm` packages can be built from this directory with the following syntax

```shell
make rpm
```

Artifacts will be located in `rpmbuild` under the following directory structure:
`rpmbuild/$distro-$distro_version/`

## Specifying a specific distro

```shell
make fedora
```

## Specifying a specific distro version
```shell
make fedora-25
```