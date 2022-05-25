include common.mk

APP_DIR:=$(realpath $(CURDIR)/..)

# Taken from: https://www.cmcrossroads.com/article/printing-value-makefile-variable
print-%  : ; @echo $($*)

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: clean
clean: ## remove build artifacts
	$(MAKE) -C rpm clean
	$(MAKE) -C deb clean
	$(MAKE) -C static clean

.PHONY: rpm
rpm: ## build rpm packages
	$(MAKE) -C $@ APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) rpm

.PHONY: deb
deb: ## build deb packages
	$(MAKE) -C $@ APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) deb

.PHONY: static
static: DOCKER_BUILD_PKGS:=static-linux cross-mac cross-win cross-arm
static: ## build static-compiled packages
	for p in $(DOCKER_BUILD_PKGS); do \
		$(MAKE) -C $@ APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) $${p} || exit 1; \
	done

.PHONY: static-linux
static-linux: ## build static-compiled packages
	$(MAKE) -C static APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) static-linux

.PHONY: cross-mac
cross-mac: ## build static-compiled packages
	$(MAKE) -C static APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) cross-mac

.PHONY: cross-win
cross-win: ## build static-compiled packages
	$(MAKE) -C static APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) cross-win

.PHONY: cross-arm
cross-arm: ## build static-compiled packages
	$(MAKE) -C static APP_DIR=$(APP_DIR) GO_VERSION=$(GO_VERSION) cross-arm
