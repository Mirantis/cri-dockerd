include ../common.mk

APP_DIR:=$(realpath $(CURDIR)/../../)
GITCOMMIT?=$(shell cd $(APP_DIR) && git rev-parse --short HEAD)
GO_BASE_IMAGE=golang
GO_IMAGE=$(GO_BASE_IMAGE):$(GO_VERSION)-bookworm
DEB_VERSION=$(shell ./gen-deb-ver $(APP_DIR) "$(VERSION)")
CHOWN:=docker run --rm -v $(CURDIR):/v -w /v alpine chown
EPOCH?=0

ifdef BUILD_IMAGE
	BUILD_IMAGE_FLAG=--build-arg BUILD_IMAGE=$(BUILD_IMAGE)
endif

COMMON_FILES=common
BUILD?=DOCKER_BUILDKIT=1 \
	docker build \
	$(BUILD_IMAGE_FLAG) \
	--build-arg GO_IMAGE=$(GO_IMAGE) \
	--build-arg COMMON_FILES=$(COMMON_FILES) \
	-t debbuild-$@/$(ARCH) \
	-f $(CURDIR)/$@/Dockerfile .
# Additional flags may be necessary at some point
RUN_FLAGS=
RUN=docker run --rm -i \
	-e EPOCH='$(EPOCH)' \
	-e DEB_VERSION=$(word 1, $(DEB_VERSION)) \
	-e VERSION=$(word 2, $(DEB_VERSION)) \
	-e LDFLAGS=$(shell echo \"${CRI_DOCKERD_LDFLAGS}\") \
	-e GITCOMMIT=$(GITCOMMIT) \
	-e PLATFORM \
	-v $(CURDIR)/debbuild/$@:/build \
	$(RUN_FLAGS) \
	debbuild-$@/$(ARCH)

SOURCE_FILES=app.tgz cri-docker.service cri-docker.socket
SOURCES=$(addprefix sources/, $(SOURCE_FILES))

DEBIAN_VERSIONS := debian-bullseye debian-bookworm
#UBUNTU_VERSIONS := ubuntu-xenial ubuntu-bionic ubuntu-cosmic ubuntu-disco ubuntu-eoan
UBUNTU_VERSIONS := ubuntu-bionic ubuntu-focal ubuntu-jammy
RASPBIAN_VERSIONS := raspbian-bullseye
DISTROS := $(DEBIAN_VERSIONS) $(UBUNTU_VERSIONS) $(RASPBIAN_VERSIONS)

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## remove build artifacts
	[ ! -d debbuild ] || $(CHOWN) -R $(shell id -u):$(shell id -g) debbuild
	$(RM) -r debbuild
	[ ! -d sources ] || $(CHOWN) -R $(shell id -u):$(shell id -g) sources
	$(RM) -r sources

.PHONY: deb
deb: ubuntu debian ## build all deb packages except for raspbian

.PHONY: ubuntu
ubuntu: $(UBUNTU_VERSIONS) ## build all ubuntu deb packages

.PHONY: debian
debian: $(DEBIAN_VERSIONS) ## build all debian deb packages

.PHONY: raspbian
raspbian: $(RASPBIAN_VERSIONS) ## build all raspbian deb packages

.PHONY: $(DISTROS)
$(DISTROS): $(SOURCES)
	@echo "== Building packages for $@ =="
	$(BUILD)
	$(RUN)
	$(CHOWN) -R $(shell id -u):$(shell id -g) debbuild/$@

sources/app.tgz:
	mkdir -p $(@D)
	docker run --rm -i -w /v \
		-v $(APP_DIR):/app \
		-v $(CURDIR)/$(@D):/v \
		alpine \
		tar -C / -c -z -f /v/app.tgz --exclude .git app

sources/cri-docker.service: ../systemd/cri-docker.service
	mkdir -p $(@D)
	cp $< $@

sources/cri-docker.socket: ../systemd/cri-docker.socket
	mkdir -p $(@D)
	cp $< $@
