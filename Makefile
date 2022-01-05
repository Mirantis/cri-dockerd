APP_DIR:=$(CURDIR)/src
PACKAGING_DIR:=$(CURDIR)/packaging

export VERSION=$(shell (git describe --abbrev=0 --tags | sed -e 's/v//') || echo $(cat VERSION)-$(git log -1 --pretty='%h'))
BUILDTIME=`date +%FT%T%z`
PRERELEASE=`grep -q dev <<< "${VERSION}" && echo "pre" || echo ""`
GITCOMMIT=`git log -1 --pretty='%h'`
export LDFLAGS=-ldflags "-X github.com/Mirantis/cri-dockerd/version.Version=${VERSION} -X github.com/Mirantis/cri-dockerd/version.PreRelease=${PRERELEASE} -X github.com/Mirantis/cri-dockerd/version.BuildTime=${BUILDTIME} -X github.com/Mirantis/cri-dockerd/version.GitCommit=${GITCOMMIT}"

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

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
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) cross-mac

.PHONY: cross-win
cross-win: ## build static packages
	$(MAKE) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) cross-win

.PHONY: clean
clean: ## clean the build artifacts
	-$(MAKE) -C $(PACKAGING_DIR) clean
