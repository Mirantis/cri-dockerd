ARCH=$(shell uname -m)
GO_VERSION:=1.18.3
PLATFORM=cri-dockerd
SHELL:=/bin/bash
VERSION?=0.2.3-dev

export PLATFORM
