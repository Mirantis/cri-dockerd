APP_DIR:=$(CURDIR)
PACKAGING_DIR:=$(CURDIR)/packaging

DATE_FMT=+%Y%m%d.%H%M%S
SOURCE_DATE_EPOCH?=$(shell git log -1 --pretty=%ct)
ifdef SOURCE_DATE_EPOCH
    BUILD_DATE?=$(shell date -u -d "@$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" "$(DATE_FMT)" 2>/dev/null || date -u "$(DATE_FMT)")
else
    BUILD_DATE?=$(shell date "$(DATE_FMT)")
endif

export VERSION?=$(shell (git describe --abbrev=0 --tags | sed -e 's/v//') || echo $(cat VERSION)-$(git log -1 --pretty='%h'))
PRERELEASE=`grep -q dev <<< "${VERSION}" && echo "pre" || echo ""`
REVISION?=`git log -1 --pretty='%h'`
export CRI_DOCKERD_LDFLAGS=-ldflags "-X github.com/Mirantis/cri-dockerd/version.Version=${VERSION} -X github.com/Mirantis/cri-dockerd/version.PreRelease=${PRERELEASE} -X github.com/Mirantis/cri-dockerd/version.BuildTime=${BUILD_DATE} -X github.com/Mirantis/cri-dockerd/version.GitCommit=${REVISION}"

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
	-$(MAKE) -C $(PACKAGING_DIR) clean
