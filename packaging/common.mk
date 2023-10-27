ARCH=$(shell uname -m)
GO_VERSION:=1.19.10
PLATFORM=cri-dockerd
SHELL:=/bin/bash
VERSION?=0.3.7-dev

export PLATFORM
