include ../common.mk

APP_DIR:=$(realpath $(CURDIR)/../../)
GO_BASE_IMAGE=golang
WINDOWS_BUILDER?=windows-engine-builder
GOPATH=C:\go
CRI_DOCKER_GOPATH=C:\gopath\src\github.com\Mirantis
CRI_DOCKER_GITCOMMIT?=$(shell git -C $APP_DIR rev-parse --short HEAD)

clean:
	-$(RM) $(WINDOWS_BUILDER)
	-$(RM) -r cri-docker
	-$(RM) -r sources
	-$(RM) *.zip
	-docker rmi -f $(WINDOWS_BUILDER)

$(WINDOWS_BUILDER):
	docker build -t $(WINDOWS_BUILDER) \
		--build-arg GO_IMAGE=$(GO_BASE_IMAGE) \
		-f Dockerfile .
	echo 1 > $@

cri-docker:
	mkdir $@

cri-docker/cri-dockerd.exe: cri-docker $(WINDOWS_BUILDER)
	docker run --rm \
		-e CRI_DOCKER_GITCOMMIT="$(CRI_DOCKER_GITCOMMIT)" \
		-e VERSION="$(VERSION)" \
		-e PLATFORM="$(PLATFORM)" \
		-v "$(CURDIR)/cri-docker:c:\out" \
		-v "$(APP_DIR):$(CRI_DOCKER_GOPATH)\cri-docker" \
		-w "$(CRI_DOCKER_GOPATH)\cri-docker" \
		--entrypoint powershell \
		$(WINDOWS_BUILDER) \
		"hack/make.ps1 -Daemon; Move-Item cri-dockerd.exe c:\out"

.PHONY: win
win: cri-docker-$(VERSION).zip

cri-docker-$(VERSION).zip: cri-docker/cri-dockerd.exe
	$(RM) $@
	docker run --rm \
		-v "$(CURDIR):C:\gopath" \
		-w "C:\gopath" \
		--entrypoint powershell \
		$(WINDOWS_BUILDER) \
		"Compress-Archive -Path C:/gopath/cri-docker -DestinationPath C:/gopath/cri-docker-$(VERSION).zip"
