include ../common.mk

APP_DIR:=$(realpath $(CURDIR)/../../)
STATIC_VERSION:=$(shell ../static/gen-static-ver $(APP_DIR) $(VERSION))
GO_BASE_IMAGE=golang
GO_IMAGE?=$(GO_BASE_IMAGE):$(GO_VERSION)-bookworm
GEN_RPM_VER=$(shell ./gen-rpm-ver $(APP_DIR) $(VERSION))
CRI_DOCKER_GITCOMMIT?=$(word 3,$(GEN_RPM_VER))
CHOWN=docker run --rm -i -v $(CURDIR):/v -w /v alpine chown

DOCKERFILE=Dockerfile
ifdef NEEDS_ARCH_SPECIFIC
	DOCKERFILE=Dockerfile.$(ARCH)
endif
ifdef BUILD_IMAGE
	BUILD_IMAGE_FLAG=--build-arg BUILD_IMAGE=$(BUILD_IMAGE)
endif
BUILD?=DOCKER_BUILDKIT=1 \
	docker build \
	$(BUILD_IMAGE_FLAG) \
	--build-arg GO_IMAGE=$(GO_IMAGE) \
	-t rpmbuild-$@/$(ARCH) \
	-f $@/$(DOCKERFILE) \
	.

SPEC_FILES?=cri-dockerd.spec
SPECS?=$(addprefix SPECS/, $(SPEC_FILES))
RPMBUILD=docker run --privileged --rm -i \
	-e PLATFORM \
	-v $(CURDIR)/rpmbuild/SOURCES:/root/rpmbuild/SOURCES \
	-v $(CURDIR)/rpmbuild/RPMS:/root/rpmbuild/RPMS \
	-v $(CURDIR)/rpmbuild/SRPMS:/root/rpmbuild/SRPMS
RPMBUILD_FLAGS?=-ba\
	--define '_gitcommit $(CRI_DOCKER_GITCOMMIT)' \
	--define '_release $(word 2,$(GEN_RPM_VER))' \
	--define '_version $(word 1,$(GEN_RPM_VER))' \
	--define '_origversion $(word 4, $(GEN_RPM_VER))' \
	--define '_buildldflags $(shell echo ${CRI_DOCKERD_LDFLAGS} | sed -e "s/-ldflags //")' \
	$(SPECS)
RUN?=$(RPMBUILD) rpmbuild-$@/$(ARCH) $(RPMBUILD_FLAGS)

SOURCE_FILES=app.tgz cri-docker.service cri-docker.socket LICENSE
SOURCES=$(addprefix rpmbuild/SOURCES/, $(SOURCE_FILES))

FEDORA_RELEASES := fedora-36 fedora-35
CENTOS_RELEASES :=

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## remove build artifacts
	[ ! -d rpmbuild ] || $(CHOWN) -R $(shell id -u):$(shell id -g) rpmbuild
	$(RM) -r rpmbuild/

.PHONY: rpm
rpm: fedora centos ## build all rpm packages

.PHONY: fedora
fedora: $(FEDORA_RELEASES) ## build all fedora rpm packages

.PHONY: centos
centos: $(CENTOS_RELEASES) ## build all centos rpm packages

.PHONY: $(FEDORA_RELEASES) $(CENTOS_RELEASES)
$(FEDORA_RELEASES) $(CENTOS_RELEASES): $(SOURCES)
	@echo "${APP_DIR}"
	@echo "${VERSION}"
	@echo "$(shell ./gen-rpm-ver $(APP_DIR) $(VERSION))"
	@echo "== Building packages for $@ =="
	$(CHOWN) -R root:root rpmbuild
	$(BUILD)
	$(RUN)
	$(CHOWN) -R $(shell id -u):$(shell id -g) rpmbuild

rpmbuild/SOURCES/app.tgz:
	mkdir -p rpmbuild/SOURCES
	docker run --rm -i -w /v \
		-v $(APP_DIR):/app \
		-v $(CURDIR)/rpmbuild/SOURCES:/v \
		alpine \
		tar -C / -c -z -f /v/app.tgz --exclude .git app

rpmbuild/SOURCES/cri-docker.service: ../systemd/cri-docker.service
	mkdir -p $(@D)
	cp $< $@

rpmbuild/SOURCES/cri-docker.socket: ../systemd/cri-docker.socket
	mkdir -p $(@D)
	cp $< $@

rpmbuild/SOURCES/LICENSE: ../../LICENSE
	mkdir -p $(@D)
	cp $< $@
