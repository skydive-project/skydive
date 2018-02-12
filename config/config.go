/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
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
)

// ErrNoAnalyzerSpecified error no analyzer section is specified in the configuration file
var ErrNoAnalyzerSpecified = errors.New("No analyzer specified in the configuration file")

var (
	cfg           *viper.Viper
	relocationMap = map[string]string{
		"auth.keystone.auth_url":               "openstack.auth_url",
		"auth.keystone.tenant_name":            "openstack.tenant_name",
		"auth.keystone.domain_name":            "openstack.domain_name",
		"agent.topology.neutron.auth_url":      "openstack.auth_url",
		"agent.topology.neutron.domain_name":   "openstack.domain_name",
		"agent.topology.neutron.endpoint_type": "openstack.endpoint_type",
		"agent.topology.neutron.password":      "openstack.password",
		"agent.topology.neutron.region_name":   "openstack.region_name",
		"agent.topology.neutron.tenant_name":   "openstack.tenant_name",
		"agent.topology.neutron.username":      "openstack.username",
	}
)

func init() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cfg = viper.New()

	cfg.SetDefault("agent.flow.probes", []string{"gopacket", "pcapsocket"})
	cfg.SetDefault("agent.flow.pcapsocket.bind_address", "127.0.0.1")
	cfg.SetDefault("agent.flow.pcapsocket.min_port", 8100)
	cfg.SetDefault("agent.flow.pcapsocket.max_port", 8132)
	cfg.SetDefault("agent.flow.stats_update", 1)
	cfg.SetDefault("agent.listen", "127.0.0.1:8081")
	cfg.SetDefault("agent.topology.probes", []string{"ovsdb"})
	cfg.SetDefault("agent.topology.netlink.metrics_update", 30)
	cfg.SetDefault("agent.topology.neutron.endpoint_type", "public")
	cfg.SetDefault("agent.X509_servername", "")

	cfg.SetDefault("analyzer.flowtable_expire", 600)
	cfg.SetDefault("analyzer.flowtable_update", 60)
	cfg.SetDefault("analyzer.listen", "127.0.0.1:8082")
	cfg.SetDefault("analyzer.replication.debug", false)
	cfg.SetDefault("analyzer.ssh_enabled", false)
	cfg.SetDefault("analyzer.storage.bulk_insert", 100)
	cfg.SetDefault("analyzer.storage.bulk_insert_deadline", 5)
	cfg.SetDefault("analyzer.topology.probes", []string{})

	cfg.SetDefault("auth.type", "noauth")
	cfg.SetDefault("auth.keystone.tenant", "admin")

	cfg.SetDefault("cache.expire", 300)
	cfg.SetDefault("cache.cleanup", 30)

	cfg.SetDefault("docker.url", "unix:///var/run/docker.sock")
	cfg.SetDefault("docker.netns.run_path", "/var/run/docker/netns")

	cfg.SetDefault("etcd.data_dir", "/var/lib/skydive/etcd")
	cfg.SetDefault("etcd.embedded", true)
	cfg.SetDefault("etcd.name", host)
	cfg.SetDefault("etcd.listen", "127.0.0.1:12379")

	cfg.SetDefault("flow.expire", 600)
	cfg.SetDefault("flow.update", 60)
	cfg.SetDefault("flow.protocol", "udp")

	cfg.SetDefault("graph.backend", "memory")

	cfg.SetDefault("http.rest.debug", false)
	cfg.SetDefault("http.ws.ping_delay", 2)
	cfg.SetDefault("http.ws.pong_timeout", 5)
	cfg.SetDefault("http.ws.bulk_maxmsgs", 100)
	cfg.SetDefault("http.ws.bulk_maxdelay", 1)
	cfg.SetDefault("http.ws.queue_size", 10000)
	cfg.SetDefault("http.ws.enable_write_compression", true)

	cfg.SetDefault("host_id", host)

	cfg.SetDefault("http.rest.debug", false)
	cfg.SetDefault("http.ws.ping_delay", 2)
	cfg.SetDefault("http.ws.pong_timeout", 5)
	cfg.SetDefault("http.ws.bulk_maxmsgs", 100)
	cfg.SetDefault("http.ws.bulk_maxdelay", 1)
	cfg.SetDefault("http.ws.queue_size", 10000)
	cfg.SetDefault("http.ws.enable_write_compression", true)

	cfg.SetDefault("k8s.config_file", "/etc/skydive/kubeconfig")
	cfg.SetDefault("k8s.probes", []string{"networkpolicy", "pod", "container", "node", "namespace"})

	cfg.SetDefault("logging.backends", []string{"stderr"})
	cfg.SetDefault("logging.color", true)
	cfg.SetDefault("logging.encoder", "")
	cfg.SetDefault("logging.file.path", "/var/log/skydive.log")
	cfg.SetDefault("logging.level", "INFO")
	cfg.SetDefault("logging.syslog.tag", "skydive")

	cfg.SetDefault("netns.run_path", "/var/run/netns")

	cfg.SetDefault("opencontrail.mpls_udp_port", 51234)

	cfg.SetDefault("ovs.ovsdb", "unix:///var/run/openvswitch/db.sock")
	cfg.SetDefault("ovs.oflow.enable", false)

	cfg.SetDefault("sflow.port_min", 6345)
	cfg.SetDefault("sflow.port_max", 6355)

	cfg.SetDefault("storage.elasticsearch.host", "127.0.0.1:9200")
	cfg.SetDefault("storage.elasticsearch.maxconns", 10)
	cfg.SetDefault("storage.elasticsearch.retry", 60)
	cfg.SetDefault("storage.elasticsearch.bulk_maxdocs", 100)
	cfg.SetDefault("storage.elasticsearch.bulk_maxdelay", 5)
	cfg.SetDefault("storage.orientdb.addr", "http://localhost:2480")
	cfg.SetDefault("storage.orientdb.database", "Skydive")
	cfg.SetDefault("storage.orientdb.username", "root")
	cfg.SetDefault("storage.orientdb.password", "root")

	cfg.SetDefault("ui.bandwidth_absolute_active", 1)
	cfg.SetDefault("ui.bandwidth_absolute_alert", 1000)
	cfg.SetDefault("ui.bandwidth_absolute_warning", 100)
	cfg.SetDefault("ui.bandwidth_relative_active", 0.1)
	cfg.SetDefault("ui.bandwidth_relative_alert", 0.8)
	cfg.SetDefault("ui.bandwidth_relative_warning", 0.5)
	cfg.SetDefault("ui.bandwidth_source", "netlink")
	cfg.SetDefault("ui.bandwidth_threshold", "relative")
	cfg.SetDefault("ui.bandwidth_update_rate", 5)

	replacer := strings.NewReplacer(".", "_", "-", "_")
	cfg.SetEnvPrefix("SKYDIVE")
	cfg.SetEnvKeyReplacer(replacer)
	cfg.AutomaticEnv()
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

func checkStrictRangeFloat(key string, min, max float64) error {
	if value := cfg.GetFloat64(key); value <= min || value > max {
		return fmt.Errorf("invalid value for %s (%f)", key, value)
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

	if err := checkPositiveInt("etcd.max_snap_files"); err != nil {
		return err
	}

	return nil
}

func checkViperSupportedExts(ext string) bool {
	for _, e := range viper.SupportedExts {
		if e == ext {
			return true
		}
	}
	return false
}

// InitConfig with a backend
func InitConfig(backend string, paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("Empty configuration path")
	}

	cfg.SetConfigType("yaml")

	switch backend {
	case "file":
		for _, path := range paths {
			configFile, err := os.Open(path)
			if err != nil {
				return err
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

	return checkConfig()
}

// GetConfig get current config
func GetConfig() *viper.Viper {
	return cfg
}

// SetDefault set default configuration key the value
func SetDefault(key string, value interface{}) {
	cfg.SetDefault(key, value)
}

// GetAnalyzerServiceAddresses returns a list of connectable Analyzers
func GetAnalyzerServiceAddresses() ([]common.ServiceAddress, error) {
	var addresses []common.ServiceAddress
	for _, a := range GetStringSlice("analyzers") {
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
	addresses, err := GetAnalyzerServiceAddresses()
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
	etcdServers := GetStringSlice("etcd.servers")
	if len(etcdServers) > 0 {
		return etcdServers
	}
	if address, err := GetOneAnalyzerServiceAddress(); err == nil {
		return []string{"http://" + address.Addr + ":12379"}
	}
	return []string{"http://localhost:12379"}
}

// IsTLSenabled returns true is the analyzer certificates are set
func IsTLSenabled() bool {
	certPEM := GetString("analyzer.X509_cert")
	keyPEM := GetString("analyzer.X509_key")
	if len(certPEM) > 0 && len(keyPEM) > 0 {
		return true
	}
	return false
}

func realKey(key string) string {
	for {
		if cfg.IsSet(key) {
			return key
		}
		newKey, found := relocationMap[key]
		if !found {
			return key
		}
		fmt.Fprintf(os.Stderr, "Config value '%s' is now deprecated. Please use '%s' instead\n", key, newKey)
		key = newKey
	}
}

// Get returns a value of the configuration as in interface
func Get(key string) interface{} {
	return cfg.Get(realKey(key))
}

// Set a value of the configuration
func Set(key string, value interface{}) {
	cfg.Set(key, value)
}

// GetBool returns a boolean from the configuration
func GetBool(key string) bool {
	return cfg.GetBool(realKey(key))
}

// GetInt returns an interger from the configuration
func GetInt(key string) int {
	return cfg.GetInt(realKey(key))
}

// GetString returns a string from the configuration
func GetString(key string) string {
	return cfg.GetString(realKey(key))
}

// GetStringSlice returns a slice of strings from the configuration
func GetStringSlice(key string) []string {
	return cfg.GetStringSlice(realKey(key))
}

// GetStringMapString returns a map of strings from the configuration
func GetStringMapString(key string) map[string]string {
	return cfg.GetStringMapString(realKey(key))
}

// BindPFlag binds a command line flag to a configuration value
func BindPFlag(key string, flag *pflag.Flag) error {
	return cfg.BindPFlag(key, flag)
}

// GetURL constructs a URL from a tuple of protocol, address, port and path
// If TLS is enabled, it will return the https (or wss) version of the URL.
func GetURL(protocol string, addr string, port int, path string) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("%s://%s:%d%s", protocol, addr, port, path))

	if (protocol == "http" || protocol == "ws") && IsTLSenabled() == true {
		u.Scheme += "s"
	}

	return u
}
