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

export VERSION?=$(shell (git describe --tags | sed -e 's/v//'))
PRERELEASE=`grep -q dev <<< "${VERSION}" && echo "pre" || echo ""`
REVISION?=`git log -1 --pretty='%h'`
export CRI_DOCKERD_LDFLAGS:=-ldflags "${CRI_DOCKERD_LDFLAGS} -s -w -buildid=${REVISION} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.Version=${VERSION} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.PreRelease=${PRERELEASE} \
	-X github.com/Mirantis/cri-dockerd/cmd/version.GitCommit=${REVISION}"

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: cri-dockerd
cri-dockerd: ## build cri-dockerd
	GOARCH=$(ARCH) go build -trimpath $(CRI_DOCKERD_LDFLAGS) -o $@

### Release
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

### Development
.PHONY: clean
clean: ## clean the build artifacts
	$(RM) cri-dockerd
	$(RM) -r build
	-$(MAKE) -C $(PACKAGING_DIR) clean

.PHONY: run
run: cri-dockerd ## Run cri-docker in a running minikube
	sudo ./cri-dockerd --log-level debug --network-plugin=""

.PHONY: dev
dev: cri-dockerd ## Run cri-docker in a running minikube
	./scripts/replace-in-minikube

#### Testing
.PHONY: integration
integration: ## Run integration tests
	sudo critest -runtime-endpoint=unix:///var/run/cri-dockerd.sock -ginkgo.skip="runtime should support apparmor|runtime should support reopening container log|runtime should support execSync with timeout|runtime should support selinux|.*should support propagation.*"

.PHONY: test
test: ## Run unit tests
	go test ./...

### Documentation
.PHONY: docs
docs: ## Run docs server
	hugo server --source docs/
