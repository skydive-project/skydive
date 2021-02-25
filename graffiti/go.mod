module github.com/skydive-project/skydive/graffiti

go 1.14

require (
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962
	github.com/abbot/go-http-auth v0.4.0
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/casbin/casbin v1.9.1
	github.com/cnf/structhash v0.0.0-20201013183111-a92e111048cd
	github.com/coreos/etcd v3.3.22+incompatible
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/fatih/structs v1.1.0
	github.com/go-test/deep v1.0.7
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.3.2
	github.com/gomatic/funcmap v1.1.0 // indirect
	github.com/gomatic/renderizer v1.0.1 // indirect
	github.com/gophercloud/gophercloud v0.13.0
	github.com/gorilla/context v1.1.1
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-version v1.2.1
	github.com/mailru/easyjson v0.7.6
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/hashstructure v1.0.0
	github.com/mitchellh/mapstructure v1.3.3
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/olivere/elastic/v7 v7.0.21
	github.com/peterh/liner v1.2.1
	github.com/pierrec/xxHash v0.1.5
	github.com/pkg/errors v0.9.1
	github.com/pmylund/go-cache v2.1.0+incompatible
	github.com/robertkrimen/otto v0.0.0-20200922221731-ef014fd054ac
	github.com/safchain/insanelock v0.0.0-20200217234559-cfbf166e05b3
	github.com/skydive-project/go-debouncer v1.0.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/cobra v1.1.1
	github.com/tchap/zapext v1.0.0
	github.com/xeipuuv/gojsonschema v1.2.0
	go.uber.org/zap v1.16.0
	golang.org/x/net v0.0.0-20201021035429-f5854403a974
	golang.org/x/tools v0.0.0-20210106214847-113979e3529a
	gopkg.in/sourcemap.v1 v1.0.5 // indirect
	gopkg.in/yaml.v2 v2.3.0
	sigs.k8s.io/yaml v1.2.0 // indirect
)

replace (
	github.com/coreos/bbolt => go.etcd.io/bbolt v1.3.5
	github.com/skydive-project/skydive/graffiti => ./graffiti
	github.com/spf13/viper v1.4.0 => github.com/lebauce/viper v0.0.0-20190903114911-3b7a98e30843
)
