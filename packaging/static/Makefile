include ../common.mk

APP_DIR:=$(realpath $(CURDIR)/../../)
STATIC_VERSION:=$(shell ./gen-static-ver $(APP_DIR) $(VERSION))
CHOWN=docker run --rm -v $(CURDIR):/v -w /v alpine chown
HASH_CMD=docker run -v $(CURDIR):/sum -w /sum debian:jessie bash hash_files
DIR_TO_HASH:=build/linux
GO_VERSION=$(shell grep "ARG GO_VERSION" $(APP_DIR)/dockerfiles/Dockerfile.dev | awk -F'=' '{print $$2}')
APP_GOLANG_IMG=golang:$(GO_VERSION)

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
	mkdir -p build/linux/cri-docker
	cd $(APP_DIR) && go get && env GOOS=linux GOARCH=amd64 go build -o cri-dockerd
	mv $(APP_DIR)/cri-dockerd build/linux/cri-dockerd
	tar -C build/linux -c -z -f build/linux/cri-docker-$(STATIC_VERSION).tgz cri-docker

.PHONY: hash_files
hash_files:
	@echo "Hashing directory $(DIR_TO_HASH)"
	$(HASH_CMD) "$(DIR_TO_HASH)"

.PHONY: cross-mac
cross-mac:
	mkdir -p build/mac/cri-docker
	cd $(APP_DIR) && go get && env GOOS=darwin GOARCH=amd64 go build -o cri-dockerd-darwin-amd64
	mv $(APP_DIR)/cri-dockerd-darwin-amd64 build/mac/cri-dockerd
	tar -C build/mac -c -z -f build/mac/cri-docker-$(STATIC_VERSION).tgz cri-docker

.PHONY: cross-win
cross-win:
	mkdir -p build/win/cri-docker
	cd $(APP_DIR) && go get && env GOOS=windows GOARCH=amd64 go build -o cri-dockerd-windows-amd64
	mv $(APP_DIR)/cri-dockerd-windows-amd64 build/win/cri-docker/cri-dockerd.exe
	docker run --rm -v $(CURDIR)/build/win:/v -w /v alpine sh -c 'apk update && apk add zip && zip -r cri-docker-$(STATIC_VERSION).zip cri-docker'
	$(CHOWN) -R $(shell id -u):$(shell id -g) build

.PHONY: cross-arm
cross-arm: ## create tgz with linux armhf client only
	mkdir -p build/arm/cri-docker
	cd $(APP_DIR) && go get && env GOOS=linux GOARCH=arm64 go build -o cri-dockerd-arm64
	mv $(APP_DIR)/cri-dockerd-arm64 build/arm/cri-dockerd
	tar -C build/arm -c -z -f build/arm/cri-docker-$(STATIC_VERSION).tgz cri-docker