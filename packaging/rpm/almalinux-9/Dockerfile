ARG GO_IMAGE
ARG DISTRO=almalinux
ARG SUITE=9
ARG BUILD_IMAGE=${DISTRO}:${SUITE}

FROM ${GO_IMAGE} AS golang

FROM ${BUILD_IMAGE}
ARG DISTRO
ARG SUITE
ENV DISTRO=${DISTRO}
ENV SUITE=${SUITE}
ENV GOPROXY=direct
ENV GOPATH=/go
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV AUTO_GOPATH 1
ENV DOCKER_BUILDTAGS seccomp selinux
ENV RUNC_BUILDTAGS seccomp selinux
RUN yum install -y rpm-build rpmlint yum-utils
# Enable CodeReady Builder repository to get device-mapper-devel and glibc-static.
RUN dnf config-manager --set-enabled crb
COPY SPECS /root/rpmbuild/SPECS
RUN yum-builddep -y /root/rpmbuild/SPECS/*.spec
COPY --from=golang /usr/local/go /usr/local/go/
WORKDIR /root/rpmbuild
ENTRYPOINT ["/bin/rpmbuild"]
