ARCH=$(shell uname -m)
BUILDTIME=$(shell date -u -d "@$${SOURCE_DATE_EPOCH:-$$(date +%s)}" --rfc-3339 ns 2> /dev/null | sed -e 's/ /T/')
DEFAULT_PRODUCT_LICENSE:=Community Engine
GO_VERSION:=1.13.11
PLATFORM=Docker Engine - Community
SHELL:=/bin/bash
VERSION?=0.0.0-dev

export BUILDTIME
export PLATFORM
