#debuginfo not supported with Go
%global debug_package %{nil}
%global gopath      %{_datadir}/gocode

# docker_version is the version of docker requires by packages
%global docker_version 1.8.2
# openvswitch_version is the version of openvswitch requires by packages
%global openvswitch_version 2.3.1

Name:           skydive
Version:        0.2.0
Release:        1%{?dist}
Summary:        Real-time network topology and protocols analyzer.
License:        ASL 2.0
URL:            https://github.com/redhat-cip/skydive
ExclusiveArch:  x86_64
Source0:        https://github.com/redhat-cip/skydive/archive/%{commit}/%{name}-%{version}.tar.gz
BuildRequires:  golang >= 1.4
BuildRequires:  systemd

%description
Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of what is happening in the network
infrastrure.

Skydive agents collect topology informations and flows and forward them to a
central agent for further analysis. All the informations a stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows informations. Currently only the Neutron driver is provided
but more drivers will come soon.

%package analyzer
Summary:          Skydive analyzer
Requires:         %{name} = %{version}-%{release}
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd

%description analyzer
Collects data captured by the Skydive agents.

%package agent
Summary:          Skydive agent
Requires:         %{name} = %{version}-%{release}
Requires:         openvswitch >= %{openvswitch_version}
Requires(post):   systemd
Requires(preun):  systemd
Requires(postun): systemd

%description agent
The Skydive agent has to be started on each node where the topology and
flows informations will be captured.

%prep
%setup -q

%build
export GOPATH=`pwd`/Godeps/_workspace
mkdir Godeps/_workspace/src/github.com/redhat-cip
ln -s `pwd` Godeps/_workspace/src/github.com/redhat-cip/skydive
cd Godeps/_workspace/src/github.com/redhat-cip/skydive
go install -v ./...

%install
install -d %{buildroot}%{_bindir}

install -p -m 755 Godeps/_workspace/bin/skydive %{buildroot}%{_bindir}/skydive
for bin in agent analyzer
do
  install -D -m 644 contrib/systemd/skydive-${bin}.service %{buildroot}%{_unitdir}/skydive-${bin}.service
done

install -D -m 644 etc/skydive.yml.default %{buildroot}/%{_sysconfdir}/skydive/skydive.yml

%post agent
%systemd_post %{basename:%{name}-agent.service}

%preun agent
%systemd_preun %{basename:%{name}-agent.service}

%postun agent
%systemd_postun

%post analyzer
%systemd_post %{basename:%{name}-analyzer.service}

%preun analyzer
%systemd_preun %{basename:%{name}-analyzer.service}

%postun analyzer
%systemd_postun

%files
%defattr(-,root,root,-)
%doc README.md LICENSE
%{_bindir}/skydive
%config(noreplace) %{_sysconfdir}/skydive/skydive.yml

%files agent
%defattr(-,root,root,-)
%{_unitdir}/skydive-agent.service

%files analyzer
%defattr(-,root,root,-)
%{_unitdir}/skydive-analyzer.service

%changelog
* Wed Apr 27 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.3.0-1
- Bump to version 0.3.0

* Fri Mar 25 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.2.0-1
- Bump to version 0.2.0

* Mon Feb 1 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.1.0-1
- Initial release of RPM
