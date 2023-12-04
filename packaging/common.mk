ARCH=$(shell uname -m)
GO_VERSION?=$(shell grep GO_VERSION ../.github/.env | grep -v HUGO_VERSION | cut -d '=' -f 2)
PLATFORM=cri-dockerd
SHELL:=/bin/bash
export VERSION?=$(shell (git describe --tags))

export PLATFORM
