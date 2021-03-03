%global debug_package %{nil}


Name: cri-docker
Version: %{_version}
Release: %{_release}%{?dist}
Epoch: 3
Source0: app.tgz
Source1: cri-docker.service
Source2: cri-docker.socket
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
Requires: iptables or nftables
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
BuildRequires: btrfs-progs-devel
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
Docker is a product for you to build, ship and run any application as a
lightweight container.

Docker containers are both hardware-agnostic and platform-agnostic. This means
they can run anywhere, from your laptop to the largest cloud compute instance and
everything in between - and they don't require you to use a particular
language, framework or packaging system. That makes them great building blocks
for deploying and scaling web apps, databases, and backend services without
depending on a particular stack or provider.

%prep
%setup -q -c -n src -a 0

%build

export CRI_DOCKER_GITCOMMIT=%{_gitcommit}
mkdir -p /go/src/github.com/evol262
ln -s /root/rpmbuild/BUILD/src/app /go/src/github.com/evol262/cri-docker

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

%post
%systemd_post cri-docker.service

%preun
%systemd_preun cri-docker.service

%postun
%systemd_postun_with_restart cri-docker.service

%changelog
