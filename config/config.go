/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package config

import (
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	// Viper remote client need to be internally initialized
	_ "github.com/spf13/viper/remote"

	"github.com/skydive-project/skydive/common"
	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
)

// ErrNoAnalyzerSpecified error no analyzer section is specified in the configuration file
var ErrNoAnalyzerSpecified = errors.New("No analyzer specified in the configuration file")

var (
	cfg           *SkydiveConfig
	relocationMap = map[string][]string{
		"agent.auth.api.backend":               {"auth.type"},
		"agent.auth.cluster.password":          {"auth.analyzer_password"},
		"agent.auth.cluster.username":          {"auth.analyzer_username"},
		"agent.capture.stats_update":           {"agent.flow.stats_update"},
		"agent.flow.sflow.bind_address":        {"sflow.bind_address"},
		"agent.flow.sflow.port_min":            {"sflow.port_min"},
		"agent.flow.sflow.port_max":            {"sflow.port_max"},
		"agent.topology.docker.url":            {"docker.url"},
		"agent.topology.docker.netns.run_path": {"docker.netns.run_path"},
		"agent.topology.netns.run_path":        {"netns.run_path"},
		"analyzer.auth.api.backend":            {"auth.type"},
		"analyzer.auth.cluster.backend":        {"auth.type"},
		"analyzer.auth.cluster.password":       {"auth.analyzer_password"},
		"analyzer.auth.cluster.username":       {"auth.analyzer_username"},
		"analyzer.flow.backend":                {"analyzer.storage.backend"},
		"analyzer.flow.max_buffer_size":        {"analyzer.storage.max_flow_buffer_size"},
		"analyzer.topology.backend":            {"graph.backend"},
		"analyzer.topology.k8s.config_file":    {"k8s.config_file"},
		"analyzer.topology.k8s.probes":         {"k8s.probes"},
	}
)

// Config defines a config interface
type Config interface {
	SetDefault(key string, value interface{})
	IsSet(key string) bool
	Get(key string) interface{}
	Set(key string, value interface{})
	GetBool(key string) bool
	GetInt(key string) int
	GetString(key string) string
	GetStringSlice(key string) []string
	GetStringMapString(key string) map[string]string
	GetStringMap(key string) map[string]interface{}
}

// SkydiveConfig implements the Config interface
type SkydiveConfig struct {
	*viper.Viper
}

func init() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cfg = &SkydiveConfig{
		Viper: viper.New(),
	}

	cfg.SetDefault("agent.auth.api.backend", "noauth")
	cfg.SetDefault("agent.capture.stats_update", 1)
	cfg.SetDefault("agent.flow.probes", []string{"gopacket", "pcapsocket"})
	cfg.SetDefault("agent.flow.netflow.bind_address", "127.0.0.1")
	cfg.SetDefault("agent.flow.netflow.port_min", 6365)
	cfg.SetDefault("agent.flow.netflow.port_max", 6375)
	cfg.SetDefault("agent.flow.pcapsocket.bind_address", "127.0.0.1")
	cfg.SetDefault("agent.flow.pcapsocket.min_port", 8100)
	cfg.SetDefault("agent.flow.pcapsocket.max_port", 8132)
	cfg.SetDefault("agent.flow.ebpf.polling_rate", 16000)
	cfg.SetDefault("agent.flow.sflow.bind_address", "127.0.0.1")
	cfg.SetDefault("agent.flow.sflow.port_min", 6345)
	cfg.SetDefault("agent.flow.sflow.port_max", 6355)
	cfg.SetDefault("agent.listen", "127.0.0.1:8081")
	cfg.SetDefault("agent.topology.probes", []string{"ovsdb"})
	cfg.SetDefault("agent.topology.blockdev.lsblk_path", "/usr/bin/lsblk")
	cfg.SetDefault("agent.topology.docker.url", "unix:///var/run/docker.sock")
	cfg.SetDefault("agent.topology.docker.netns.run_path", "/var/run/docker/netns")
	cfg.SetDefault("agent.topology.netlink.metrics_update", 30)
	cfg.SetDefault("agent.topology.netns.run_path", "/var/run/netns")
	cfg.SetDefault("agent.topology.neutron.domain_name", "Default")
	cfg.SetDefault("agent.topology.neutron.endpoint_type", "public")
	cfg.SetDefault("agent.topology.neutron.ssl_insecure", false)
	cfg.SetDefault("agent.topology.neutron.region_name", "RegionOne")
	cfg.SetDefault("agent.topology.neutron.tenant_name", "service")
	cfg.SetDefault("agent.topology.neutron.username", "neutron")
	cfg.SetDefault("agent.topology.runc.run_path", []string{"/run/containerd/runc", "/run/runc", "/run/runc-ctrs"})
	cfg.SetDefault("agent.topology.socketinfo.host_update", 10)
	cfg.SetDefault("agent.topology.vpp.connect", "")
	cfg.SetDefault("agent.topology.bess.host", "127.0.0.1")
	cfg.SetDefault("agent.topology.bess.port", 10514)

	cfg.SetDefault("analyzer.auth.cluster.backend", "noauth")
	cfg.SetDefault("analyzer.auth.api.backend", "noauth")
	cfg.SetDefault("analyzer.flow.backend", "memory")
	cfg.SetDefault("analyzer.flow.max_buffer_size", 100000)
	cfg.SetDefault("analyzer.listen", "127.0.0.1:8082")
	cfg.SetDefault("analyzer.topology.backend", "memory")
	cfg.SetDefault("analyzer.topology.probes", []string{})
	cfg.SetDefault("analyzer.topology.k8s.config_file", "/etc/skydive/kubeconfig")
	cfg.SetDefault("analyzer.topology.ovn.address", "unix:///var/run/openvswitch/ovnnb_db.sock")
	cfg.SetDefault("analyzer.topology.istio.config_file", "/etc/skydive/kubeconfig")

	cfg.SetDefault("auth.basic.type", "basic") // defined for backward compatibility
	cfg.SetDefault("auth.keystone.tenant_name", "admin")
	cfg.SetDefault("auth.keystone.type", "keystone") // defined for backward compatibility
	cfg.SetDefault("auth.keystone.domain_name", "Default")
	cfg.SetDefault("auth.noauth.type", "noauth") // defined for backward compatibility

	cfg.SetDefault("cache.expire", 300)
	cfg.SetDefault("cache.cleanup", 30)

	cfg.SetDefault("etcd.client_timeout", etcdclient.DefaultTimeout)
	cfg.SetDefault("etcd.data_dir", "/var/lib/skydive/etcd")
	cfg.SetDefault("etcd.embedded", true)
	cfg.SetDefault("etcd.listen", fmt.Sprintf("%s:%d", etcdclient.DefaultServer, etcdclient.DefaultPort))
	cfg.SetDefault("etcd.name", host)

	cfg.SetDefault("flow.expire", 600)
	cfg.SetDefault("flow.update", 60)
	cfg.SetDefault("flow.max_entries", 500000)
	cfg.SetDefault("flow.protocol", "udp")
	cfg.SetDefault("flow.application_timeout.arp", 10)
	cfg.SetDefault("flow.application_timeout.dns", 10)

	cfg.SetDefault("host_id", host)

	cfg.SetDefault("http.rest.debug", false)
	cfg.SetDefault("http.ws.ping_delay", 2)
	cfg.SetDefault("http.ws.pong_timeout", 5)
	cfg.SetDefault("http.ws.queue_size", 10000)
	cfg.SetDefault("http.ws.enable_write_compression", true)

	cfg.SetDefault("logging.backends", []string{"stderr"})
	cfg.SetDefault("logging.color", true)
	cfg.SetDefault("logging.encoder", "")
	cfg.SetDefault("logging.file.path", "/var/log/skydive.log")
	cfg.SetDefault("logging.level", "INFO")
	cfg.SetDefault("logging.syslog.address", "/dev/log")
	cfg.SetDefault("logging.syslog.protocol", "unixgram")
	cfg.SetDefault("logging.syslog.tag", "skydive")

	cfg.SetDefault("opencontrail.host", "localhost")
	cfg.SetDefault("opencontrail.mpls_udp_port", 51234)
	cfg.SetDefault("opencontrail.port", 8085)

	cfg.SetDefault("ovs.ovsdb", "unix:///var/run/openvswitch/db.sock")
	cfg.SetDefault("ovs.oflow.enable", false)
	cfg.SetDefault("ovs.oflow.openflow_versions", []string{"OpenFlow10", "OpenFlow11", "OpenFlow12", "OpenFlow13", "OpenFlow14"})
	cfg.SetDefault("ovs.enable_stats", false)

	cfg.SetDefault("rbac.model.request_definition", []string{"sub, obj, act"})
	cfg.SetDefault("rbac.model.policy_definition", []string{"sub, obj, act, eft"})
	cfg.SetDefault("rbac.model.role_definition", []string{"_, _"})
	cfg.SetDefault("rbac.model.policy_effect", []string{"some(where (p_eft == allow)) && !some(where (p_eft == deny))"})
	cfg.SetDefault("rbac.model.matchers", []string{"g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act"})

	cfg.SetDefault("storage.elasticsearch.driver", "elasticsearch")  // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.elasticsearch.host", "127.0.0.1:9200")   // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.elasticsearch.bulk_maxdelay", 5)         // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.elasticsearch.index_age_limit", 0)       // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.elasticsearch.index_entries_limit", 0)   // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.elasticsearch.indices_to_keep", 0)       // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.memory.driver", "memory")                // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.orientdb.driver", "orientdb")            // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.orientdb.addr", "http://localhost:2480") // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.orientdb.database", "Skydive")           // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.orientdb.username", "root")              // defined for backward compatibility and to set defaults
	cfg.SetDefault("storage.orientdb.password", "root")              // defined for backward compatibility and to set defaults

	cfg.SetDefault("ui", map[string]interface{}{})

	replacer := strings.NewReplacer(".", "_", "-", "_")
	cfg.SetEnvPrefix("SKYDIVE")
	cfg.SetEnvKeyReplacer(replacer)
	cfg.AutomaticEnv()
	cfg.SetTypeByDefaultValue(true)
}

func checkStrictPositiveInt(key string) error {
	if value := cfg.GetInt(key); value <= 0 {
		return fmt.Errorf("invalid value for %s (%d)", key, value)
	}

	return nil
}

func checkPositiveInt(key string) error {
	if value := cfg.GetInt(key); value < 0 {
		return fmt.Errorf("invalid value for %s (%d)", key, value)
	}

	return nil
}

func checkConfig() error {
	if err := checkStrictPositiveInt("flow.expire"); err != nil {
		return err
	}

	if err := checkStrictPositiveInt("flow.update"); err != nil {
		return err
	}

	if err := checkPositiveInt("etcd.max_wal_files"); err != nil {
		return err
	}

	return checkPositiveInt("etcd.max_snap_files")
}

func setStorageDefaults() {
	for key := range cfg.GetStringMap("storage") {
		if key == "elasticsearch" || key == "orientdb" || key == "memory" {
			continue
		}

		driver := cfg.GetString("storage." + key + ".driver")
		if driver == "" {
			continue
		}

		for defkey, value := range cfg.GetStringMap("storage." + driver) {
			cfg.SetDefault("storage."+key+"."+defkey, value)
		}
	}
}

// LoadConfig initializes config with the given backend/path
func LoadConfig(cfg *viper.Viper, backend string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("Empty configuration path")
	}

	cfg.SetConfigType("yaml")

	switch backend {
	case "file":
		for _, path := range paths {
			configFile := os.Stdin
			if path != "-" {
				var err error
				configFile, err = os.Open(path)
				if err != nil {
					return err
				}
			}

			if err := cfg.MergeConfig(configFile); err != nil {
				return err
			}
		}
	case "etcd":
		if len(paths) != 1 {
			return fmt.Errorf("You can specify only one etcd endpoint for configuration")
		}
		path := paths[0]
		u, err := url.Parse(path)
		if err != nil {
			return err
		}
		if err := cfg.AddRemoteProvider("etcd", fmt.Sprintf("%s://%s", u.Scheme, u.Host), u.Path); err != nil {
			return err
		}
		if err := cfg.ReadRemoteConfig(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("Invalid backend: %s", backend)
	}

	return nil
}

// InitConfig global config
func InitConfig(backend string, paths []string) error {
	if err := LoadConfig(cfg.Viper, backend, paths); err != nil {
		return err
	}

	setStorageDefaults()

	return checkConfig()
}

// GetConfig get current config
func GetConfig() *SkydiveConfig {
	return cfg
}

// SetDefault set the default configuration value for a key
func SetDefault(key string, value interface{}) {
	cfg.SetDefault(key, value)
}

// SetDefault set the default configuration value for a key
func (c *SkydiveConfig) SetDefault(key string, value interface{}) {
	c.Viper.SetDefault(key, value)
}

// GetAnalyzerServiceAddresses returns a list of connectable Analyzers
func GetAnalyzerServiceAddresses() ([]common.ServiceAddress, error) {
	return cfg.GetAnalyzerServiceAddresses()
}

// GetAnalyzerServiceAddresses returns a list of connectable Analyzers
func (c *SkydiveConfig) GetAnalyzerServiceAddresses() ([]common.ServiceAddress, error) {
	var addresses []common.ServiceAddress
	for _, a := range c.GetStringSlice("analyzers") {
		sa, err := common.ServiceAddressFromString(a)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, sa)
	}

	// shuffle in order to load balance connections
	for i := range addresses {
		j := rand.Intn(i + 1)
		addresses[i], addresses[j] = addresses[j], addresses[i]
	}

	return addresses, nil
}

// GetOneAnalyzerServiceAddress returns a random connectable Analyzer
func GetOneAnalyzerServiceAddress() (common.ServiceAddress, error) {
	return cfg.GetOneAnalyzerServiceAddress()
}

// GetOneAnalyzerServiceAddress returns a random connectable Analyzer
func (c *SkydiveConfig) GetOneAnalyzerServiceAddress() (common.ServiceAddress, error) {
	addresses, err := c.GetAnalyzerServiceAddresses()
	if err != nil {
		return common.ServiceAddress{}, err
	}

	if len(addresses) == 0 {
		return common.ServiceAddress{}, ErrNoAnalyzerSpecified
	}

	return addresses[rand.Intn(len(addresses))], nil
}

// GetEtcdServerAddrs returns the ETCD server address specified in the configuration file or embedded
func GetEtcdServerAddrs() []string {
	return cfg.GetEtcdServerAddrs()
}

// GetEtcdServerAddrs returns the ETCD server address specified in the configuration file or embedded
func (c *SkydiveConfig) GetEtcdServerAddrs() []string {
	etcdServers := c.GetStringSlice("etcd.servers")
	if len(etcdServers) > 0 {
		return etcdServers
	}

	port := etcdclient.DefaultPort
	if c.GetBool("etcd.embedded") {
		sa, err := common.ServiceAddressFromString(c.GetString("etcd.listen"))
		if err == nil {
			port = sa.Port
		}
	}

	if address, err := c.GetOneAnalyzerServiceAddress(); err == nil {
		return []string{fmt.Sprintf("http://%s:%d", address.Addr, port)}
	}
	return []string{fmt.Sprintf("http://%s:%d", etcdclient.DefaultServer, port)}
}

// IsTLSEnabled returns true is the client / server certificates are set
func IsTLSEnabled() bool {
	return cfg.IsTLSEnabled()
}

// IsTLSEnabled returns true is the client / server certificates are set
func (c *SkydiveConfig) IsTLSEnabled() bool {
	client := c.GetString("tls.client_cert")
	clientKey := c.GetString("tls.client_key")
	server := c.GetString("tls.server_cert")
	serverKey := c.GetString("tls.server_key")
	ca := c.GetString("tls.ca_cert")

	if len(client) > 0 &&
		len(clientKey) > 0 &&
		len(server) > 0 &&
		len(serverKey) > 0 &&
		len(ca) > 0 {
		return true
	}

	return false
}

func realKey(key string) string {
	if cfg.IsSet(key) {
		return key
	}

	// check is there is a deprecated key that can be used
	depKeys, found := relocationMap[key]
	if !found {
		return key
	}
	for _, depKey := range depKeys {
		if cfg.IsSet(depKey) {
			fmt.Fprintf(os.Stderr, "Config value '%s' is now deprecated. Please use '%s' instead\n", depKey, key)
			return depKey
		}
	}

	return key
}

// IsSet returns wether a key is set
func IsSet(key string) bool {
	return cfg.IsSet(key)
}

// IsSet returns wether a key is set
func (c *SkydiveConfig) IsSet(key string) bool {
	return c.Viper.IsSet(key)
}

// Get returns a value of the configuration as in interface
func Get(key string) interface{} {
	return cfg.Get(key)
}

// Get returns a value of the configuration as in interface
func (c *SkydiveConfig) Get(key string) interface{} {
	return c.Viper.Get(realKey(key))
}

// Set a value of the configuration
func Set(key string, value interface{}) {
	cfg.Set(key, value)
}

// Set a value of the configuration
func (c *SkydiveConfig) Set(key string, value interface{}) {
	c.Viper.Set(key, value)
}

// GetBool returns a boolean from the configuration
func GetBool(key string) bool {
	return cfg.GetBool(key)
}

// GetBool returns a boolean from the configuration
func (c *SkydiveConfig) GetBool(key string) bool {
	return c.Viper.GetBool(realKey(key))
}

// GetInt returns an interger from the configuration
func GetInt(key string) int {
	return cfg.GetInt(key)
}

// GetInt returns an interger from the configuration
func (c *SkydiveConfig) GetInt(key string) int {
	return c.Viper.GetInt(realKey(key))
}

// GetString returns a string from the configuration
func GetString(key string) string {
	return cfg.GetString(key)
}

// GetString returns a string from the configuration
func (c *SkydiveConfig) GetString(key string) string {
	return c.Viper.GetString(realKey(key))
}

// GetStringSlice returns a slice of strings from the configuration
func GetStringSlice(key string) []string {
	return cfg.GetStringSlice(key)
}

// GetStringSlice returns a slice of strings from the configuration
func (c *SkydiveConfig) GetStringSlice(key string) []string {
	return c.Viper.GetStringSlice(realKey(key))
}

// GetStringMapString returns a map of strings from the configuration
func GetStringMapString(key string) map[string]string {
	return cfg.GetStringMapString(key)
}

// GetStringMapString returns a map of strings from the configuration
func (c *SkydiveConfig) GetStringMapString(key string) map[string]string {
	return c.Viper.GetStringMapString(realKey(key))
}

// BindPFlag binds a command line flag to a configuration value
func BindPFlag(key string, flag *pflag.Flag) error {
	return cfg.BindPFlag(key, flag)
}

// GetURL constructs a URL from a tuple of protocol, address, port and path
// If TLS is enabled, it will return the https (or wss) version of the URL.
func GetURL(protocol string, addr string, port int, path string) *url.URL {
	return cfg.GetURL(protocol, addr, port, path)
}

// GetURL constructs a URL from a tuple of protocol, address, port and path
// If TLS is enabled, it will return the https (or wss) version of the URL.
func (c *SkydiveConfig) GetURL(protocol string, addr string, port int, path string) *url.URL {
	return common.MakeURL(protocol, addr, port, path, c.IsTLSEnabled())
}
