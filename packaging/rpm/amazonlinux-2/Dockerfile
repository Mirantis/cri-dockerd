ARG GO_IMAGE
ARG BUILD_IMAGE=amazonlinux:2
FROM ${GO_IMAGE} as golang

FROM ${BUILD_IMAGE}
ENV DISTRO amazonlinux
ENV SUITE 2
ENV GOPATH=/go
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV AUTO_GOPATH 1
ENV DOCKER_BUILDTAGS seccomp selinux
ENV RUNC_BUILDTAGS seccomp selinux
RUN yum install -y rpm-build rpmlint yum-utils
COPY SPECS /root/rpmbuild/SPECS
RUN yum-builddep -y /root/rpmbuild/SPECS/*.spec
COPY --from=golang /usr/local/go /usr/local/go/
WORKDIR /root/rpmbuild
ENTRYPOINT ["/bin/rpmbuild"]
