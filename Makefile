APP_DIR:=$(CURDIR)
PACKAGING_DIR:=$(CURDIR)/packaging
VERSION=$(shell cat VERSION)

.PHONY: help
help: ## show make targets
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {sub("\\\\n",sprintf("\n%22c"," "), $$2);printf " \033[36m%-20s\033[0m  %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: deb
deb: ## build deb packages
	$(MAKE) VERSION=$(VERSION) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) deb

.PHONY: rpm
rpm: ## build rpm packages
	$(MAKE) VERSION=$(VERSION) APP_DIR=$(APP_DIR)  -C $(PACKAGING_DIR) rpm

.PHONY: static
static: ## build static packages
	$(MAKE) VERSION=$(VERSION) APP_DIR=$(APP_DIR) -C $(PACKAGING_DIR) static

.PHONY: clean
clean: ## clean the build artifacts
	-$(MAKE) -C $(PACKAGING_DIR) clean
