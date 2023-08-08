APP_DIR:=$(CURDIR)
PACKAGING_DIR:=$(CURDIR)/packaging
RELEASE_DIR:=$(CURDIR)/build/release

DATE_FMT=+%Y%m%d.%H%M%S
SOURCE_DATE_EPOCH?=$(shell git log -1 --pretty=%ct)
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE?=$(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE?=$(shell date "$(DATE_FMT)")
endif

SHELL=/bin/bash

export VERSION?=$(shell (git describe --abbrev=0 --tags | sed -e 's/v//') || echo $(cat VERSION)-$(git log -1 --pretty='%h'))
PRERELEASE=`grep -q dev <<< "${VERSION}" && echo "pre" || echo ""`
REVISION?=`git log -1 --pretty='%h'`
export CRI_DOCKERD_LDFLAGS=-ldflags "-s -w -buildid=${REVISION} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.Version=${VERSION} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.PreRelease=${PRERELEASE} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.GitCommit=${REVISION}"

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: cri-dockerd
cri-dockerd: ## build cri-dockerd
	GOARCH=$(ARCH) go build -trimpath $(CRI_DOCKERD_LDFLAGS) -o $@

.PHONY: deb
deb: ## build deb packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) deb

.PHONY: rpm
rpm: ## build rpm packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) rpm

.PHONY: static
static: ## build static packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) static

.PHONY: static-linux
static-linux: ## build static packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) static-linux

.PHONY: cross-mac
cross-mac: ## build static packages
	echo ${SOURCE_DATE_EPOCH}
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) cross-mac

.PHONY: cross-win
cross-win: ## build static packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) cross-win

.PHONY: cross-arm
cross-arm: ## build static packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) cross-arm

.PHONY: clean
clean: ## clean the build artifacts
	$(RM) cri-dockerd
	$(RM) -r build
	-$(MAKE) -C $(PACKAGING_DIR) clean

.PHONY: release
release: static-linux deb rpm cross-arm cross-mac cross-win ## build the release binaries
	mkdir -p $(RELEASE_DIR)

	# Copy the release files
	# Debian
	cp $(PACKAGING_DIR)/deb/debbuild/debian-bullseye/cri-dockerd_$(VERSION)~3-0~debian-bullseye_amd64.deb $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/deb/debbuild/debian-buster/cri-dockerd_$(VERSION)~3-0~debian-buster_amd64.deb $(RELEASE_DIR)
	# Ubuntu
	cp $(PACKAGING_DIR)/deb/debbuild/ubuntu-bionic/cri-dockerd_$(VERSION)~3-0~ubuntu-bionic_amd64.deb $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/deb/debbuild/ubuntu-focal/cri-dockerd_$(VERSION)~3-0~ubuntu-focal_amd64.deb $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/deb/debbuild/ubuntu-jammy/cri-dockerd_$(VERSION)~3-0~ubuntu-jammy_amd64.deb $(RELEASE_DIR)
	# CentOS
	cp $(PACKAGING_DIR)/rpm/rpmbuild/RPMS/x86_64/cri-dockerd-$(VERSION).*.el7.x86_64.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/RPMS/x86_64/cri-dockerd-$(VERSION).*.el8.x86_64.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/SRPMS/cri-dockerd-$(VERSION).*.el7.src.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/SRPMS/cri-dockerd-$(VERSION).*.el8.src.rpm $(RELEASE_DIR)
	# Fedora
	cp $(PACKAGING_DIR)/rpm/rpmbuild/RPMS/x86_64/cri-dockerd-$(VERSION).*.fc35.x86_64.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/RPMS/x86_64/cri-dockerd-$(VERSION).*.fc36.x86_64.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/SRPMS/cri-dockerd-$(VERSION).*.fc35.src.rpm $(RELEASE_DIR)
	cp $(PACKAGING_DIR)/rpm/rpmbuild/SRPMS/cri-dockerd-$(VERSION).*.fc36.src.rpm $(RELEASE_DIR)
	# arm
	cp $(PACKAGING_DIR)/static/build/arm/cri-dockerd-$(VERSION).tgz $(RELEASE_DIR)/cri-dockerd-$(VERSION).arm64.tgz
	# win
	cp $(PACKAGING_DIR)/static/build/win/cri-dockerd-$(VERSION).zip $(RELEASE_DIR)/cri-dockerd-$(VERSION).win.amd64.zip
	# mac
	cp $(PACKAGING_DIR)/static/build/mac/cri-dockerd-$(VERSION).tgz $(RELEASE_DIR)/cri-dockerd-$(VERSION).darwin.amd64.tgz
	# linux
	cp $(PACKAGING_DIR)/static/build/linux/cri-dockerd-$(VERSION).tgz $(RELEASE_DIR)/cri-dockerd-$(VERSION).amd64.tgz

.PHONY: dev
dev: cri-dockerd ## Run cri-docker in a running minikube
	./scripts/replace-in-minikube
.PHONY: docs
docs:
	hugo server --source docs/

