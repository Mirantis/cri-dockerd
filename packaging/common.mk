ARCH=$(shell uname -m)
GO_VERSION:=1.16.8
PLATFORM=cri-dockerd
SHELL:=/bin/bash
VERSION?=0.2.0-dev

export PLATFORM
