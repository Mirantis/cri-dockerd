ARCH=$(shell uname -m)
BUILDTIME=$(shell date -u -d "@$${SOURCE_DATE_EPOCH:-$$(date +%s)}" --rfc-3339 ns 2> /dev/null | sed -e 's/ /T/')
GO_VERSION:=1.13.11
PLATFORM=cri-dockerd
SHELL:=/bin/bash
VERSION?=0.1.0-dev

export BUILDTIME
export PLATFORM
