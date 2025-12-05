include ../common.mk

APP_DIR:=$(realpath $(CURDIR)/../../)
CGO_ENABLED ?= 0
CHOWN=docker run --rm -v $(CURDIR):/v -w /v alpine chown
HASH_CMD=docker run -v $(CURDIR):/sum -w /sum debian:jessie bash hash_files
DIR_TO_HASH:=build/linux
GO_VERSION=$(shell grep "ARG GO_VERSION" $(APP_DIR)/dockerfiles/Dockerfile.dev | awk -F'=' '{print $$2}')
APP_GOLANG_IMG=golang:$(GO_VERSION)

# BUILDARCH is the host architecture
# ARCH is the target architecture
BUILDARCH ?= $(shell uname -m)

# golang names for host architecture
ifeq ($(BUILDARCH),aarch64)
        BUILDARCH=arm64
endif
ifeq ($(BUILDARCH),x86_64)
        BUILDARCH=amd64
endif
ifeq ($(BUILDARCH),armv7l)
        BUILDARCH=armv7
endif

ARCH ?= $(BUILDARCH)
ifeq ($(ARCH),aarch64)
        override ARCH=arm64
endif
ifeq ($(ARCH),armv7l)
        override ARCH=armv7
endif
ifeq ($(ARCH),x86_64)
        override ARCH=amd64
endif

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## remove build artifacts
	[ ! -d build ] || $(CHOWN) -R $(shell id -u):$(shell id -g) build
	$(RM) -r build

.PHONY: static
static: static-linux cross-mac cross-win cross-arm ## create all static packages

.PHONY: static-linux
static-linux:
	mkdir -p build/linux/cri-dockerd
	cd $(APP_DIR) && env CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=$(ARCH) go build -trimpath ${CRI_DOCKERD_LDFLAGS} -o cri-dockerd
	mv $(APP_DIR)/cri-dockerd build/linux/cri-dockerd/cri-dockerd
	tar -C build/linux -c -z -f build/linux/cri-dockerd-$(VERSION).amd64.tgz cri-dockerd

.PHONY: hash_files
hash_files:
	@echo "Hashing directory $(DIR_TO_HASH)"
	$(HASH_CMD) "$(DIR_TO_HASH)"

.PHONY: cross-mac
cross-mac:
	mkdir -p build/mac/cri-dockerd
	cd $(APP_DIR) && env CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=$(ARCH) go build -trimpath ${CRI_DOCKERD_LDFLAGS} -o cri-dockerd-darwin-$(ARCH)
	mv $(APP_DIR)/cri-dockerd-darwin-$(ARCH) build/mac/cri-dockerd/cri-dockerd
	tar -C build/mac -c -z -f build/mac/cri-dockerd-$(VERSION).darwin.$(ARCH).tgz cri-dockerd

.PHONY: cross-win
cross-win:
	mkdir -p build/win/cri-dockerd
	cd $(APP_DIR) && env CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=$(ARCH) go build -trimpath ${CRI_DOCKERD_LDFLAGS} -o cri-dockerd-windows-amd64
	mv $(APP_DIR)/cri-dockerd-windows-amd64 build/win/cri-dockerd/cri-dockerd.exe
	if ! grep -sq 'docker\|lxc' /proc/1/cgroup; then \
	    docker run --rm -v $(CURDIR)/build/win:/v -w /v alpine sh -c 'apk update && apk add zip && zip -r cri-dockerd-$(VERSION).win.amd64.zip cri-dockerd'; \
	    $(CHOWN) -R $(shell id -u):$(shell id -g) build; \
	fi

.PHONY: cross-arm
cross-arm: ## create tgz with linux arm64 client only
	mkdir -p build/arm/cri-dockerd
	cd $(APP_DIR) && env CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=arm64 go build -trimpath ${CRI_DOCKERD_LDFLAGS} -o cri-dockerd-arm64
	mv $(APP_DIR)/cri-dockerd-arm64 build/arm/cri-dockerd/cri-dockerd
	tar -C build/arm -c -z -f build/arm/cri-dockerd-$(VERSION).arm64.tgz cri-dockerd
