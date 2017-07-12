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

	"github.com/spf13/viper"
	// Viper remote client need to be internally initialized
	_ "github.com/spf13/viper/remote"

	"github.com/skydive-project/skydive/common"
)

var cfg *viper.Viper

// ErrNoAnalyzerSpecified error no analyzer section is specified in the configuration file
var (
	ErrNoAnalyzerSpecified = errors.New("No analyzer specified in the configuration file")
)

func init() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cfg = viper.New()
	cfg.SetDefault("host_id", host)
	cfg.SetDefault("agent.listen", "127.0.0.1:8081")
	cfg.SetDefault("ovs.ovsdb", "unix:///var/run/openvswitch/db.sock")
	cfg.SetDefault("graph.backend", "memory")
	cfg.SetDefault("graph.gremlin", "ws://127.0.0.1:8182")
	cfg.SetDefault("sflow.port_min", 6345)
	cfg.SetDefault("sflow.port_max", 6355)
	cfg.SetDefault("flow.expire", 600)
	cfg.SetDefault("flow.update", 60)
	cfg.SetDefault("analyzer.listen", "127.0.0.1:8082")
	cfg.SetDefault("analyzer.storage.bulk_insert", 100)
	cfg.SetDefault("analyzer.storage.bulk_insert_deadline", 5)
	cfg.SetDefault("storage.elasticsearch.host", "127.0.0.1:9200")
	cfg.SetDefault("storage.elasticsearch.maxconns", 10)
	cfg.SetDefault("storage.elasticsearch.retry", 60)
	cfg.SetDefault("storage.elasticsearch.bulk_maxdocs", 100)
	cfg.SetDefault("storage.elasticsearch.bulk_maxdelay", 5)
	cfg.SetDefault("ws_pong_timeout", 5)
	cfg.SetDefault("ws_bulk_maxmsgs", 100)
	cfg.SetDefault("ws_bulk_maxdelay", 1)
	cfg.SetDefault("docker.url", "unix:///var/run/docker.sock")
	cfg.SetDefault("netns.run_path", "/var/run/netns")
	cfg.SetDefault("etcd.data_dir", "/var/lib/skydive/etcd")
	cfg.SetDefault("etcd.embedded", true)
	cfg.SetDefault("etcd.listen", "localhost:2379")
	cfg.SetDefault("auth.type", "noauth")
	cfg.SetDefault("auth.keystone.tenant", "admin")
	cfg.SetDefault("storage.orientdb.addr", "http://localhost:2480")
	cfg.SetDefault("storage.orientdb.database", "Skydive")
	cfg.SetDefault("storage.orientdb.username", "root")
	cfg.SetDefault("storage.orientdb.password", "root")
	cfg.SetDefault("openstack.endpoint_type", "public")
	cfg.SetDefault("agent.topology.probes", []string{"ovsdb"})
	cfg.SetDefault("agent.topology.netlink.metrics_update", 30)
	cfg.SetDefault("agent.flow.probes", []string{"gopacket", "pcapsocket"})
	cfg.SetDefault("agent.flow.pcapsocket.bind_address", "127.0.0.1")
	cfg.SetDefault("agent.flow.pcapsocket.min_port", 8100)
	cfg.SetDefault("agent.flow.pcapsocket.max_port", 8132)
	cfg.SetDefault("analyzer.topology.probes", []string{})
	cfg.SetDefault("agent.X509_servername", "")
	cfg.SetDefault("opencontrail.mpls_udp_port", 51234)
	cfg.SetDefault("agent.flow.stats_update", 1)
	cfg.SetDefault("analyzer.bandwidth_source", "netlink")
	cfg.SetDefault("analyzer.bandwidth_threshold", "relative")
	cfg.SetDefault("analyzer.bandwidth_update_rate", 5)
	cfg.SetDefault("analyzer.bandwidth_relative_active", 0.1)
	cfg.SetDefault("analyzer.bandwidth_relative_warning", 0.5)
	cfg.SetDefault("analyzer.bandwidth_relative_alert", 0.8)
	cfg.SetDefault("analyzer.bandwidth_absolute_active", 1)
	cfg.SetDefault("analyzer.bandwidth_absolute_warning", 100)
	cfg.SetDefault("analyzer.bandwidth_absolute_alert", 1000)
	cfg.SetDefault("logging.backends", []string{"stderr"})
	cfg.SetDefault("logging.level", "INFO")
	cfg.SetDefault("logging.file.path", "/var/log/skydive.log")
	cfg.SetDefault("logging.format", "%{color}%{time} %{id} %{shortfile} %{shortpkg} %{longfunc} > %{level:.4s} %{id:03x}%{color:reset} %{message}")

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
	for _, a := range GetConfig().GetStringSlice("analyzers") {
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
	etcdServers := GetConfig().GetStringSlice("etcd.servers")
	if len(etcdServers) > 0 {
		return etcdServers
	}
	if addresse, err := GetOneAnalyzerServiceAddress(); err == nil {
		return []string{"http://" + addresse.Addr + ":2379"}
	}
	return []string{"http://localhost:2379"}
}

// IsTLSenabled returns true is the analyzer certificates are set
func IsTLSenabled() bool {
	certPEM := GetConfig().GetString("analyzer.X509_cert")
	keyPEM := GetConfig().GetString("analyzer.X509_key")
	if len(certPEM) > 0 && len(keyPEM) > 0 {
		return true
	}
	return false
}
