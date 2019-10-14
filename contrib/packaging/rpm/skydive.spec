%global import_path     github.com/skydive-project/skydive
%global gopath          %{_datadir}/gocode

%{!?with_features:%global with_features %{nil}}

%if !%{defined gotest}
%define gotest() go test -compiler gc -ldflags "${LDFLAGS:-}" %{?**};
%endif

%define extracttag() %(eval "echo %1 | cut -s -d '-' -f 2-")
%define extractversion() %(eval "echo %1 | cut -d '-' -f 1")
%define normalize() %(eval "echo %1 | tr '-' '.'")

%global selinuxtype targeted
%global selinux_policyver 3.13.1-192
%global moduletype contrib

%if 0%{?fedora} >= 27 || 0%{?rhel} >= 8
%global selinux_semanage_pkg policycoreutils-python-utils
%else
%global selinux_semanage_pkg policycoreutils-python
%endif

%if 0%{?fedora} >= 29 || 0%{?rhel} >= 8
%global ldflags -compressdwarf=false
%else
%global ldflags %{nil}
%endif

%if %{defined fullver}
%define vertag %extracttag %{fullver}
%if "%{vertag}" != ""
%define tag %normalize 0.%{vertag}
%endif
%endif

%{!?fullver:%global fullver 0.25.0}
%define version %{extractversion %{fullver}}
%{!?tag:%global tag 1}

Name:           skydive
Version:        %{version}
Release:        %{tag}%{?dist}
Summary:        Real-time network topology and protocols analyzer.
License:        ASL 2.0
URL:            https://%{import_path}
Source0:        https://%{import_path}/releases/download/v%{version}/skydive-%{fullver}.tar.gz
BuildRequires:  systemd
BuildRequires:  libpcap-devel libxml2-devel libvirt-devel
%if 0%{?fedora} >= 27
BuildRequires:  llvm clang kernel-headers
%endif
BuildRequires:  selinux-policy-devel, policycoreutils-devel, perl
Requires:       %{name}-selinux = %{version}-%{release}
Requires:       libpcap libxml2 libvirt-libs coreutils
Requires(pre):  /usr/sbin/useradd

# This is used by the specfile-update-bundles script to automatically
# generate the list of the Go libraries bundled into the Skydive binaries
### AUTO-BUNDLED-GEN-ENTRY-POINT

# If go_compiler is not set to 1, there is no virtual provide. Use golang instead.
BuildRequires:  %{?go_compiler:compiler(go-compiler)}%{!?go_compiler:golang} >= 1.8

%description
Skydive is an open source real-time network topology and protocols analyzer.
It aims to provide a comprehensive way of what is happening in the network
infrastrure.

Skydive agents collect topology informations and flows and forward them to a
central agent for further analysis. All the informations are stored in an
Elasticsearch database.

Skydive is SDN-agnostic but provides SDN drivers in order to enhance the
topology and flows informations.

%package analyzer
Summary:          Skydive analyzer
Requires:         %{name} = %{version}-%{release}
Requires(post):   systemd %{selinux_semanage_pkg}
Requires(preun):  systemd
Requires(postun): systemd %{selinux_semanage_pkg}

%description analyzer
Collects data captured by the Skydive agents.

%package agent
Summary:          Skydive agent
Requires:         %{name} = %{version}-%{release}
Requires(post):   systemd %{selinux_semanage_pkg}
Requires(preun):  systemd
Requires(postun): systemd %{selinux_semanage_pkg}

%description agent
The Skydive agent has to be started on each node where the topology and
flows informations will be captured.

%package ansible
Summary:          Skydive ansible recipes
Requires:         ansible
BuildArch:        noarch

%description ansible
Ansible recipes to deploy Skydive

%package selinux
Summary:          Skydive selinux recipes
Requires:         container-selinux, policycoreutils, libselinux-utils
Requires(post):   selinux-policy-base >= %{selinux_policyver}, policycoreutils
Requires(postun): policycoreutils
BuildArch:        noarch

%description selinux
This package installs and sets up the SELinux policy security module for Skydive.

%prep
%setup -q -n skydive-%{fullver}/src/%{import_path}

%build
export GOPATH=%{_builddir}/skydive-%{fullver}
make build VERSION=%{fullver} LDFLAGS=%{ldflags} GO111MODULE=off %{with_features}
%{_builddir}/skydive-%{fullver}/bin/skydive bash-completion

# SELinux build
%if 0%{?fedora} >= 27
cp contrib/packaging/rpm/skydive.te{.fedora,}
%endif
%if 0%{?rhel} >= 7
cp contrib/packaging/rpm/skydive.te{.rhel,}
%endif
make -f /usr/share/selinux/devel/Makefile -C contrib/packaging/rpm/ skydive.pp
bzip2 contrib/packaging/rpm/skydive.pp

%install
install -D -p -m 755 %{_builddir}/skydive-%{fullver}/src/%{import_path}/skydive %{buildroot}%{_bindir}/skydive
ln -s skydive %{buildroot}%{_bindir}/skydive-cli
for bin in agent analyzer
do
  install -D -m 644 contrib/systemd/skydive-${bin}.service %{buildroot}%{_unitdir}/skydive-${bin}.service
  install -D -m 644 contrib/packaging/rpm/skydive-${bin}.sysconfig %{buildroot}/%{_sysconfdir}/sysconfig/skydive-${bin}
done
install -D -m 644 etc/skydive.yml.default %{buildroot}/%{_sysconfdir}/skydive/skydive.yml
install -D -m 644 skydive-bash-completion.sh %{buildroot}/%{_sysconfdir}/bash_completion.d/skydive-bash-completion.sh
install -d -m 755 %{buildroot}/%{_datadir}/skydive-ansible
cp -R contrib/ansible/* %{buildroot}/%{_datadir}/skydive-ansible/
install -d -m 755 %{buildroot}/%{_datadir}/doc/skydive-ansible

# SELinux
install -D -m 644 contrib/packaging/rpm/skydive.pp.bz2 %{buildroot}%{_datadir}/selinux/packages/skydive.pp.bz2
install -D -m 644 contrib/packaging/rpm/skydive.if %{buildroot}%{_datadir}/selinux/devel/include/contrib/skydive.if
install -D -m 644 contrib/packaging/rpm/skydive-selinux.8 %{buildroot}%{_mandir}/man8/skydive-selinux.8

%pre
/usr/sbin/groupadd -r skydive >/dev/null 2>&1 || :
/usr/sbin/useradd -r -g skydive -d /var/lib/skydive -s /sbin/nologin \
    -c "Skydive user" skydive >/dev/null 2>&1 || :

%post agent
if %{_sbindir}/selinuxenabled && [ "$1" = "1" ] ; then
    set +e
    %{_sbindir}/semanage port -a -t skydive_agent_sflow_ports_t -p udp 6343
    %{_sbindir}/semanage port -a -t skydive_agent_sflow_ports_t -p udp 6345-6355
    %{_sbindir}/semanage port -a -t skydive_agent_pcapsocket_ports_t -p tcp 8100-8132
fi
%systemd_post %{basename:%{name}-agent.service}

%preun agent
%systemd_preun %{basename:%{name}-agent.service}

%postun agent
%systemd_postun
if %{_sbindir}/selinuxenabled && [ "$1" = "0" ] ; then
    set +e
    %{_sbindir}/semanage port -d -t skydive_agent_sflow_ports_t -p udp 6343
    %{_sbindir}/semanage port -d -t skydive_agent_sflow_ports_t -p udp 6345-6355
    %{_sbindir}/semanage port -d -t skydive_agent_pcapsocket_ports_t -p tcp 8100-8132
fi

%post analyzer
if %{_sbindir}/selinuxenabled && [ "$1" = "1" ] ; then
    set +e
    %{_sbindir}/semanage port -a -t skydive_etcd_ports_t -p tcp 12379-12380
    %{_sbindir}/semanage port -a -t skydive_analyzer_db_connect_ports_t -p tcp 2480
    %if 0%{?fedora} < 27 && 0%{?rhel} < 8
        # already set by wap_wsp_port_t
        %{_sbindir}/semanage port -a -t skydive_analyzer_db_connect_ports_t -p tcp 9200
    %endif
fi
%systemd_post %{basename:%{name}-analyzer.service}

%preun analyzer
%systemd_preun %{basename:%{name}-analyzer.service}

%postun analyzer
%systemd_postun
if %{_sbindir}/selinuxenabled && [ "$1" = "0" ] ; then
    set +e
    %{_sbindir}/semanage port -d -t skydive_etcd_ports_t -p tcp 12379-12380
    %{_sbindir}/semanage port -d -t skydive_analyzer_db_connect_ports_t -p tcp 2480
    %if 0%{?fedora} < 27 && 0%{?rhel} < 8
        # already set by wap_wsp_port_t
        %{_sbindir}/semanage port -d -t skydive_analyzer_db_connect_ports_t -p tcp 9200
    %endif
fi

%pre selinux
%selinux_relabel_pre -s %{selinuxtype}

%post selinux
%selinux_modules_install -s %{selinuxtype} %{_datadir}/selinux/packages/%{name}.pp.bz2

%postun selinux
if [ "$1" = "0" ]; then
    %selinux_modules_uninstall -s %{name}
fi

%posttrans selinux
%selinux_relabel_post -s %{selinuxtype}

%check
%{buildroot}%{_bindir}/skydive version | grep -q "skydive github.com/skydive-project/skydive %{fullver}" || exit 1

%if 0%{?with_check} && 0%{?with_unit_test} && 0%{?with_devel}
%gotest $(go list ./... | grep -v '/tests' | grep -v '/vendor/')
%endif

%files
%doc README.md LICENSE CHANGELOG.md
%{_bindir}/skydive
%{_bindir}/skydive-cli
%{_sysconfdir}/bash_completion.d/skydive-bash-completion.sh
%config(noreplace) %{_sysconfdir}/skydive/skydive.yml

%files agent
%config(noreplace) %{_sysconfdir}/sysconfig/skydive-agent
%{_unitdir}/skydive-agent.service

%files analyzer
%config(noreplace) %{_sysconfdir}/sysconfig/skydive-analyzer
%{_unitdir}/skydive-analyzer.service

%files ansible
%{_datadir}/skydive-ansible

%files selinux
%attr(0644,root,root) %{_datadir}/selinux/packages/%{name}.pp.bz2
%attr(0644,root,root) %{_datadir}/selinux/devel/include/%{moduletype}/%{name}.if
%attr(0644,root,root) %{_mandir}/man8/skydive-selinux.8.*

%changelog
* Tue Sep 10 2019 Sylvain Baubeau <sbaubeau@redhat.com> - 0.25.0-1
- Bump to version 0.25.0

* Mon Jun 24 2019 Sylvain Baubeau <sbaubeau@redhat.com> - 0.24.0-1
- Bump to version 0.24.0

* Wed May 15 2019 Sylvain Baubeau <sbaubeau@redhat.com> - 0.23.0-1
- Bump to version 0.23.0

* Thu Mar 21 2019 Sylvain Afchain <safchain@redhat.com> - 0.22.0-1
- Bump to version 0.22.0

* Fri Nov 30 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.21.0-1
- Add dependency on libvirt
- Remove skydive-ansible dependency on skydive
- Bump to version 0.21.0

* Mon Oct 8 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.20.0-1
- Bump to version 0.20.0

* Thu Sep 13 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.19.1-1
- Bump to version 0.19.1

* Wed Aug 8 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.19.0-1
- Bump to version 0.19.0

* Mon Jun 18 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.18.0-1
- Bump to version 0.18.0
- Add SElinux policy

* Tue Apr 03 2018 Sylvain Afchain <safchain@redhat.com> - 0.17.0-1
- Bump to version 0.17.0

* Mon Jan 29 2018 Sylvain Baubeau <sbaubeau@redhat.com> - 0.16.0-1
- Bump to version 0.16.0

* Tue Dec 5 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.15.0-1
- Bump to version 0.15.0

* Tue Nov 14 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.14.0-1
- Bump to version 0.14.0

* Wed Oct 11 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.13.0-1
- Bump to version 0.13.0
- Add skydive-ansible subpackage

* Fri Jul 28 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.12.0-1
- Bump to version 0.12.0

* Fri May 5 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.11.0-1
- Bump to version 0.11.0

* Thu Mar 30 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.10.0-1
- Bump to version 0.10.0

* Fri Jan 27 2017 Sylvain Baubeau <sbaubeau@redhat.com> - 0.9.0-1
- Bump to version 0.9.0
- Use Fedora golang macros and guidelines for packaging

* Fri Dec 9 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.8.0-1
- Bump to version 0.8.0

* Tue Nov 8 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.7.0-1
- Bump to version 0.7.0

* Thu Oct 6 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.6.0-1
- Bump to version 0.6.0

* Thu Sep 15 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.5.0-1
- Bump to version 0.5.0

* Thu Aug 4 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.4.0-1
- Bump to version 0.4.0

* Fri Jul 29 2016 Nicolas Planel <nplanel@redhat.com> - 0.3.0-2
- Update spec file to use govendor on go version >=1.5

* Wed Apr 27 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.3.0-1
- Bump to version 0.3.0

* Fri Mar 25 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.2.0-1
- Bump to version 0.2.0

* Mon Feb 1 2016 Sylvain Baubeau <sbaubeau@redhat.com> - 0.1.0-1
- Initial release of RPM
