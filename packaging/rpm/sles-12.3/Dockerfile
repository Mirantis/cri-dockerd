ARG GO_IMAGE
ARG ENGINE_IMAGE
ARG BUILD_IMAGE=mirantiseng/sles:12.3
FROM ${GO_IMAGE} as golang

FROM ${BUILD_IMAGE}
ENV DISTRO sles
ENV SUITE 12.3
ENV GOPATH=/go
ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin
ENV AUTO_GOPATH 1
ENV DOCKER_BUILDTAGS seccomp selinux
ENV RUNC_BUILDTAGS seccomp selinux
RUN zypper install -y rpm-build rpmlint
COPY SPECS /root/rpmbuild/SPECS
RUN zypper -n install $(rpmspec -P /root/rpmbuild/SPECS/docker-ee*.spec 2>/dev/null | sed -e '/^BuildRequires:/!d' -e 's/^BuildRequires: //' | xargs)
# suse puts the default build dir as /usr/src/rpmbuild
# to keep everything simple we just change the default
RUN echo "%_topdir    /root/rpmbuild" > /root/.rpmmacros
COPY --from=golang /usr/local/go /usr/local/go/
WORKDIR /root/rpmbuild
# hey look I'm sles and I put installed binaries somewhere different...
ENTRYPOINT ["/usr/bin/rpmbuild"]
