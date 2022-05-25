ARCH=$(shell uname -m)
GO_VERSION:=1.18.2
PLATFORM=cri-dockerd
SHELL:=/bin/bash
VERSION?=0.2.1-dev

export PLATFORM
