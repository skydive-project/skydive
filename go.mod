module github.com/skydive-project/skydive

require (
	git.fd.io/govpp.git v0.0.0-20190321220742-345201eedce4
	github.com/GehirnInc/crypt v0.0.0-20170404120257-5a3fafaa7c86
	github.com/Knetic/govaluate v0.0.0-20171022003610-9aa49832a739 // indirect
	github.com/VerizonDigital/vflow v0.0.0-20190111005900-eb30d936249e
	github.com/abbot/go-http-auth v0.4.0
	github.com/aktau/github-release v0.7.2
	github.com/bennyscetbun/jsongo v0.0.0-20190110163710-9624bef8c57b // indirect
	github.com/casbin/casbin v0.0.0-20181031010332-5ff5a6f5e38a
	github.com/cenk/hub v0.0.0-20160527103212-11382a9960d3 // indirect
	github.com/cenk/rpc2 v0.0.0-20160427170138-7ab76d2e88c7 // indirect
	github.com/cenkalti/rpc2 v0.0.0-20180727162946-9642ea02d0aa // indirect
	github.com/cnf/structhash v0.0.0-20170702194520-7710f1f78fb9
	github.com/coreos/etcd v3.3.15+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/digitalocean/go-libvirt v0.0.0-20190715144809-7b622097a793
	github.com/docker/docker v1.13.1
	github.com/ebay/go-ovn v0.0.0-20190726163905-ca0da4d10c52
	github.com/fatih/structs v0.0.0-20171020064819-f5faa72e7309
	github.com/fortytw2/leaktest v1.3.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gima/govalid v0.0.0-20150214172340-7b486932bea2
	github.com/go-swagger/go-swagger v0.20.1
	github.com/go-test/deep v0.0.0-20180509200213-57af0be209c5
	github.com/gobwas/httphead v0.0.0-20171016043908-01c9b01b368a // indirect
	github.com/gobwas/pool v0.0.0-20170829094749-32dbaa12caca // indirect
	github.com/gobwas/ws v0.0.0-20171112092802-915eed324002 // indirect
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.3.2
	github.com/golangci/golangci-lint v1.18.0
	github.com/gomatic/funcmap v0.0.0-20190110133044-62047470c142 // indirect
	github.com/gomatic/renderizer v1.0.1
	github.com/google/gopacket v1.1.17
	github.com/gophercloud/gophercloud v0.3.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/handlers v1.4.2
	github.com/gorilla/mux v1.7.0
	github.com/gorilla/websocket v1.4.1
	github.com/gosuri/uitable v0.0.0-20160404203958-36ee7e946282 // indirect
	github.com/hashicorp/go-version v0.0.0-20180322230233-23480c066577
	github.com/hashicorp/golang-lru v0.5.3
	github.com/hydrogen18/stoppableListener v0.0.0-20151210151943-dadc9ccc400c
	github.com/intel-go/nff-go v0.8.0
	github.com/iovisor/gobpf v0.0.0-20190329163444-e0d8d785d368 // indirect
	github.com/jbowtie/gokogiri v0.0.0-20190301021639-37f655d3078f // indirect
	github.com/jteeuwen/go-bindata v0.0.0-20180305030458-6025e8de665b
	github.com/juju/clock v0.0.0-20190205081909-9c5c9712527c // indirect
	github.com/juju/errors v0.0.0-20190806202954-0232dcc7464d // indirect
	github.com/juju/retry v0.0.0-20180821225755-9058e192b216 // indirect
	github.com/juju/testing v0.0.0-20190723135506-ce30eb24acd2 // indirect
	github.com/juju/utils v0.0.0-20180820210520-bf9cc5bdd62d // indirect
	github.com/juju/version v0.0.0-20180108022336-b64dbd566305 // indirect
	github.com/juju/webbrowser v0.0.0-20160309143629-54b8c57083b4 // indirect
	github.com/kami-zh/go-capturer v0.0.0-20171211120116-e492ea43421d
	github.com/kardianos/osext v0.0.0-20160811001526-c2c54e542fb7
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/libvirt/libvirt-go v0.0.0-20181005092746-9c5bdce3c18f
	github.com/lunixbochs/struc v0.0.0-20180408203800-02e4c2afbb2a
	github.com/lxc/lxd v0.0.0-20171219222704-9907f3a64b6b
	github.com/mailru/easyjson v0.7.0
	github.com/mattn/go-runewidth v0.0.0-20160315040712-d6bea18f7897 // indirect
	github.com/mattn/goveralls v0.0.2
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/hashstructure v0.0.0-20170116052023-ab25296c0f51
	github.com/mitchellh/mapstructure v1.1.2
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/networkservicemesh/networkservicemesh/controlplane/api v0.2.0
	github.com/networkservicemesh/networkservicemesh/k8s v0.0.0-20191017074247-aa5815869b2c
	github.com/newtools/ebpf v0.0.0-20190820102627-8b7eaed02eb9
	github.com/nimbess/nimbess-agent v0.0.0-20190919205041-4e6f317ac4fd
	github.com/nlewo/contrail-introspect-cli v0.0.0-20181003135217-0407b60f2edd
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/olivere/elastic v0.0.0-20190204160516-f82cf7c66881
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7 // indirect
	github.com/peterh/liner v0.0.0-20160615113019-8975875355a8
	github.com/pierrec/xxHash v0.0.0-20190318091927-d17cb990ad2d
	github.com/pmylund/go-cache v0.0.0-20170722040110-a3647f8e31d7
	github.com/robertkrimen/otto v0.0.0-20161004124959-bf1c3795ba07
	github.com/safchain/ethtool v0.0.0-20190326074333-42ed695e3de8
	github.com/safchain/insanelock v0.0.0-20180509135444-33bca4586648
	github.com/shirou/gopsutil v2.18.12+incompatible
	github.com/sirupsen/logrus v1.4.2
	github.com/skydive-project/dede v0.0.0-20180704100832-90df8e39b679
	github.com/skydive-project/goloxi v0.0.0-20190117172159-db2324197a3e
	github.com/socketplane/libovsdb v0.0.0-20160607151822-5113f8fb4d9d
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.4.0
	github.com/t-yuki/gocover-cobertura v0.0.0-20180217150009-aaee18c8195c
	github.com/tchap/zapext v0.0.0-20180117141735-e61c0c882339
	github.com/tebeka/selenium v0.0.0-20170314201507-657e45ec600f
	github.com/tomnomnom/linkheader v0.0.0-20180905144013-02ca5825eb80 // indirect
	github.com/vishvananda/netlink v1.0.0
	github.com/vishvananda/netns v0.0.0-20190625233234-7109fa855b0f
	github.com/voxelbrain/goptions v0.0.0-20180630082107-58cddc247ea2 // indirect
	github.com/weaveworks/tcptracer-bpf v0.0.0-20170817155301-e080bd747dc6
	github.com/xeipuuv/gojsonpointer v0.0.0-20170225233418-6fe8760cad35 // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20150808065054-e02fc20de94c // indirect
	github.com/xeipuuv/gojsonschema v0.0.0-20180618132009-1d523034197f
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20191011191535-87dc89f01550 // indirect
	golang.org/x/net v0.0.0-20191014212845-da9a3fd4c582
	golang.org/x/sys v0.0.0-20191010194322-b09406accb47
	golang.org/x/tools v0.0.0-20191017151554-a3bc800455d5
	google.golang.org/genproto v0.0.0-20190926190326-7ee9db18f195 // indirect
	google.golang.org/grpc v1.23.1
	gopkg.in/errgo.v1 v1.0.0-20161222125816-442357a80af5 // indirect
	gopkg.in/fsnotify/fsnotify.v1 v1.0.0-20180110053347-c2828203cd70
	gopkg.in/httprequest.v1 v1.0.0-20180209163514-93f8fee4081f // indirect
	gopkg.in/macaroon-bakery.v2 v2.0.0-20180209090814-22c04a94d902 // indirect
	gopkg.in/macaroon.v2 v2.0.0-20171017153037-bed2a428da6e // indirect
	gopkg.in/mgo.v2 v2.0.0-20190816093944-a6b53ec6cb22 // indirect
	gopkg.in/sourcemap.v1 v1.0.0-20160602085544-eef8f47ab679 // indirect
	gopkg.in/urfave/cli.v2 v2.0.0-20170928224240-b2bf3c5abeb9 // indirect
	gopkg.in/validator.v2 v2.0.0-20160201165114-3e4f037f12a1
	gopkg.in/yaml.v2 v2.2.4
	istio.io/api v0.0.0-20191029012234-9fe6a7da3673
	istio.io/client-go v0.0.0-20191024204624-13a7366c1cab
	k8s.io/api v0.0.0
	k8s.io/apimachinery v0.0.0
	k8s.io/client-go v11.0.0+incompatible
)

replace (
	github.com/census-instrumentation/opencensus-proto v0.1.0-0.20181214143942-ba49f56771b8 => github.com/census-instrumentation/opencensus-proto v0.0.3-0.20181214143942-ba49f56771b8
	github.com/digitalocean/go-libvirt => github.com/lebauce/go-libvirt v0.0.0-20190717144624-7799d804f7e4
	github.com/iovisor/gobpf => github.com/lebauce/gobpf v0.0.0-20190909090614-f9e9df81702a
	github.com/networkservicemesh/networkservicemesh => github.com/networkservicemesh/networkservicemesh v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/controlplane => github.com/networkservicemesh/networkservicemesh/controlplane v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/controlplane/api => github.com/networkservicemesh/networkservicemesh/controlplane/api v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/dataplane => github.com/networkservicemesh/networkservicemesh/dataplane v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/dataplane/api => github.com/networkservicemesh/networkservicemesh/dataplane/api v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/k8s => github.com/networkservicemesh/networkservicemesh/k8s v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/k8s/api => github.com/networkservicemesh/networkservicemesh/k8s/api v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/pkg => github.com/networkservicemesh/networkservicemesh/pkg v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/sdk => github.com/networkservicemesh/networkservicemesh/sdk v0.0.0-20191017074247-aa5815869b2c
	github.com/networkservicemesh/networkservicemesh/utils => github.com/networkservicemesh/networkservicemesh/utils v0.0.0-20191017074247-aa5815869b2c
	github.com/newtools/ebpf => github.com/nplanel/ebpf v0.0.0-20190918123742-99947faabce5
	github.com/prometheus/client_golang => github.com/prometheus/client_golang v0.9.3
	github.com/skydive-project/skydive/scripts/gendecoder => ./scripts/gendecoder
	github.com/spf13/viper v1.4.0 => github.com/lebauce/viper v0.0.0-20190903114911-3b7a98e30843
	github.com/vishvananda/netlink v1.0.0 => github.com/lebauce/netlink v0.0.0-20190122103356-fa328be7c8d2
	golang.org/x/sys => golang.org/x/sys v0.0.0-20190412213103-97732733099d
	golang.org/x/tools => golang.org/x/tools v0.0.0-20190925230517-ea99b82c7b93
	k8s.io/api => k8s.io/api v0.0.0-20191016110408-35e52d86657a
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20191016113550-5357c4baaf65
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20191004115801-a2eda9f80ab8
	k8s.io/apiserver => k8s.io/apiserver v0.0.0-20191016112112-5190913f932d
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.0.0-20191016114015-74ad18325ed5
	k8s.io/client-go => k8s.io/client-go v0.0.0-20191016111102-bec269661e48
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.0.0-20191016115326-20453efc2458
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20191016115129-c07a134afb42
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20191004115455-8e001e5d1894
	k8s.io/component-base => k8s.io/component-base v0.0.0-20191016111319-039242c015a9
	k8s.io/cri-api => k8s.io/cri-api v0.0.0-20190828162817-608eb1dad4ac
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.0.0-20191016115521-756ffa5af0bd
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.0.0-20191016112429-9587704a8ad4
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.0.0-20191016114939-2b2b218dc1df
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.0.0-20191016114407-2e83b6f20229
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.0.0-20191016114748-65049c67a58b
	k8s.io/kubectl => k8s.io/kubectl v0.0.0-20191016120415-2ed914427d51
	k8s.io/kubelet => k8s.io/kubelet v0.0.0-20191016114556-7841ed97f1b2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.0.0-20191016115753-cf0698c3a16b
	k8s.io/metrics => k8s.io/metrics v0.0.0-20191016113814-3b1a734dba6e
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.0.0-20191016112829-06bb3c9d77c9
)

go 1.11
