%global debug_package %{nil}


Name: cri-dockerd
Version: %{_version}
Release: %{_release}%{?dist}
Epoch: 3
Source0: app.tgz
Source1: cri-docker.service
Source2: cri-docker.socket
Source3: LICENSE
Summary: A CRI interface for Docker
Group: Tools/Docker
License: ASL 2.0
URL: https://www.docker.com
Vendor: Docker
Packager: Docker <support@docker.com>

Requires: container-selinux >= 2:2.74
Requires: libseccomp >= 2.3
Requires: systemd
%if 0%{?rhel} >= 8
Requires: (iptables or nftables)
%else
Requires: iptables
%endif
%if %{undefined suse_version}
Requires: libcgroup
%endif
Requires: containerd.io >= 1.2.2-3
Requires: tar
Requires: xz

# Resolves: rhbz#1165615
Requires: device-mapper-libs >= 1.02.90-1

BuildRequires: bash
BuildRequires: ca-certificates
BuildRequires: cmake
BuildRequires: device-mapper-devel
BuildRequires: gcc
BuildRequires: git
BuildRequires: glibc-static
BuildRequires: libseccomp-devel
BuildRequires: libselinux-devel
BuildRequires: libtool
BuildRequires: libtool-ltdl-devel
BuildRequires: make
BuildRequires: pkgconfig
BuildRequires: pkgconfig(systemd)
BuildRequires: selinux-policy-devel
BuildRequires: systemd-devel
BuildRequires: tar
BuildRequires: which

%description
cri-dockerd is a lightweight implementation of the CRI specification which talks to docker.

%prep
%setup -q -c -n src -a 0

%build
cp %{_topdir}/SOURCES/LICENSE /root/rpmbuild/BUILD/src/LICENSE
export CRI_DOCKER_GITCOMMIT=%{_gitcommit}
mkdir -p /go/src/github.com/Mirantis
ln -s /root/rpmbuild/BUILD/src/app /go/src/github.com/Mirantis/cri-dockerd
cd /root/rpmbuild/BUILD/src/app
GOPROXY="https://proxy.golang.org" GO111MODULE=on go build -ldflags "%{_buildldflags}"

%check
app/cri-dockerd --version

%install
# install daemon binary
install -D -p -m 0755 $(readlink -f app/cri-dockerd) $RPM_BUILD_ROOT/%{_bindir}/cri-dockerd

# install systemd scripts
install -D -m 0644 %{_topdir}/SOURCES/cri-docker.service $RPM_BUILD_ROOT/%{_unitdir}/cri-docker.service
install -D -m 0644 %{_topdir}/SOURCES/cri-docker.socket $RPM_BUILD_ROOT/%{_unitdir}/cri-docker.socket

%files
/%{_bindir}/cri-dockerd
/%{_unitdir}/cri-docker.service
/%{_unitdir}/cri-docker.socket
%license LICENSE

%post
%systemd_post cri-docker.service

%preun
%systemd_preun cri-docker.service

%postun
%systemd_postun_with_restart cri-docker.service

%changelog
