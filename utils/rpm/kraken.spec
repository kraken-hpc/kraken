Name:           kraken
Version:        1.0
Release:        0%{?dist}
Summary:        Kraken is a distributed state engine for scalable system boot and automation
Group:          Applications/System
License:        BSD-3
URL:            https://github.com/hpc/kraken
Source0:        %{name}-%{version}.tar.gz

BuildRequires:  go, golang >= 1.15, golang-bin, golang-src

%define  debug_package %{nil}

%if "%{_arch}" == "x86_64"
%define GoBuildArch amd64
%endif
%if "%{_arch}" == "aarch64"
%define GoBuildArch arm64
%endif
%if "%{_arch}" == "ppc"
%define GoBuildArch ppc
%endif
%if "%{_arch}" == "ppc64"
%define GoBuildArch ppc64
%endif
# TODO - add more architectures

%description
Kraken is a distributed state engine that can maintain state across a large set of computers. It was designed to provide full-lifecycle maintenance of HPC compute clusters, from cold boot to ongoing system state maintenance and automation.

# Define initramfs-<arch> sub-package
%package initramfs-%{GoBuildArch}
BuildArch: noarch
Group: Applications/System
Summary: A base initramfs for use with Kraken PXE configurations (%{GoBuildArch}).
%description initramfs-%{GoBuildArch}
This package installs a pre-built base initramfs (arch: %{GoBuildArch}) for use with a Kraken PXE setup.  This initramfs should be layered with at least two other pieces: 1. a set of needed system modules; 2. a set of configuration files (e.g. uinit.script).


%prep
%setup -q
cp -p %{?KrakenConfig}%{?!KrakenConfig:kraken.yaml} build.yaml

%build

rpm -D "KrakenWorkingDirectory %{?KrakenWorkingDirectory}%{?!KrakenWorkingDirectory:/}" --eval "$(cat utils/rpm/kraken.service)" > kraken.service
rpm --eval "$(cat utils/rpm/kraken.environment)" > kraken.environment

cat << EOF >> build.yaml 

targets:
  'rpm':
    os: 'linux'
    arch: '%{GoBuildArch}'
  'u-root':
EOF

go run kraken-build.go -force -v -config build.yaml

# build initramfs
export GOPATH=%{_builddir}/go
mkdir -p $GOPATH/src/github.com/hpc
ln -s $PWD $GOPATH/src/github.com/hpc/kraken
bash utils/layer0/buildlayer0_uroot.sh -o initramfs-base-%{GoBuildArch}.gz %{GoBuildArch}

chmod -R u+w $GOPATH
rm -rf $GOPATH

%install
mkdir -p %{buildroot}
install -D -m 0755 build/kraken-rpm %{buildroot}%{_sbindir}/kraken
install -D -m 0644 kraken.service %{buildroot}%{_unitdir}/kraken.service
install -D -m 0644 kraken.environment %{buildroot}%{_sysconfdir}/sysconfig/kraken
install -D -m 0644 utils/rpm/state.json %{buildroot}%{_sysconfdir}/kraken/state.json
install -D -m 0644 initramfs-base-%{GoBuildArch}.gz %{buildroot}/tftp/initramfs-base-%{GoBuildArch}.gz

%files
%defattr(-,root,root)
%license LICENSE
%{_sbindir}/kraken
%config(noreplace) %{_sysconfdir}/kraken/state.json
%{_unitdir}/kraken.service
%config(noreplace) %{_sysconfdir}/sysconfig/kraken

%files initramfs-%{GoBuildArch}
/tftp/initramfs-base-%{GoBuildArch}.gz

%changelog

* Wed Jan 13 2021 J. Lowell Wofford <lowell@lanl.gov> 1.0-0
- Initial RPM build of kraken
