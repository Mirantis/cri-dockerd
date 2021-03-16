ARG GO_IMAGE
ARG BUILD_IMAGE=oraclelinux:7
FROM ${GO_IMAGE} as golang

FROM ${BUILD_IMAGE}
ENV DISTRO oraclelinux
ENV SUITE 7
ENV GOPATH=/go
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV AUTO_GOPATH 1
ENV DOCKER_BUILDTAGS seccomp selinux
ENV RUNC_BUILDTAGS seccomp selinux
RUN yum install -y rpm-build rpmlint
COPY SPECS /root/rpmbuild/SPECS
RUN yum-builddep --enablerepo=ol7_optional_latest -y /root/rpmbuild/SPECS/*.spec
COPY --from=golang /usr/local/go /usr/local/go/
WORKDIR /root/rpmbuild
ENTRYPOINT ["/bin/rpmbuild"]
