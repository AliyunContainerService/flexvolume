%global KUBE_VERSION 1.7.2
%global RPM_RELEASE 0
%global ARCH amd64

Name: disk
Version: %{KUBE_VERSION}
Release: %{RPM_RELEASE}
Summary: Kubernetes flexvolume driver alicloud disk.
License: ASL 2.0

URL: https://kubernetes.io
Source0: http://kube-zju.oss-cn-hangzhou.aliyuncs.com/binary/amd64/1.7.2/disk

BuildRequires: curl

%description -n disk
Kubernetes flexvolume driver alicloud disk.

%prep
# Assumes the builder has overridden sourcedir to point to directory
# with this spec file. (where these files are stored) Copy them into
# the builddir so they can be installed.
# This is a useful hack for faster Docker builds when working on the spec or
# with locally obtained sources.

cp -p %SOURCE0 %{_builddir}/


%install

install -m 755 -d %{buildroot}%{_bindir}
install -m 755 -d %{buildroot}%{_prefix}/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/
install -p -m 755 -t %{buildroot}%{_prefix}/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/ disk

%files -n disk
%{_prefix}/libexec/kubernetes/kubelet-plugins/volume/exec/alicloud~disk/disk

%doc


%changelog
