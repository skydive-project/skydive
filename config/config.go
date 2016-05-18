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
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	_ "github.com/spf13/viper/remote"
)

var cfg *viper.Viper

func init() {
	cfg = viper.New()
	cfg.SetDefault("agent.analyzers", "127.0.0.1:8082")
	cfg.SetDefault("agent.listen", "127.0.0.1:8081")
	cfg.SetDefault("agent.flowtable_expire", 300)
	cfg.SetDefault("agent.flowtable_update", 30)
	cfg.SetDefault("ovs.ovsdb", "127.0.0.1:6400")
	cfg.SetDefault("graph.backend", "memory")
	cfg.SetDefault("graph.gremlin", "ws://127.0.0.1:8182")
	cfg.SetDefault("sflow.bind_address", "127.0.0.1:6345")
	cfg.SetDefault("sflow.port_min", 6345)
	cfg.SetDefault("sflow.port_max", 6355)
	cfg.SetDefault("analyzer.listen", "127.0.0.1:8082")
	cfg.SetDefault("analyzer.flowtable_expire", 600)
	cfg.SetDefault("analyzer.flowtable_update", 60)
	cfg.SetDefault("storage.elasticsearch", "127.0.0.1:9200")
	cfg.SetDefault("ws_pong_timeout", 5)
	cfg.SetDefault("docker.url", "unix:///var/run/docker.sock")
	cfg.SetDefault("etcd.data_dir", "/tmp/skydive-etcd")
	cfg.SetDefault("etcd.embedded", true)
	cfg.SetDefault("etcd.port", 2379)
	cfg.SetDefault("etcd.servers", []string{"http://127.0.0.1:2379"})
	cfg.SetDefault("auth.type", "noauth")
	cfg.SetDefault("auth.keystone.tenant", "admin")
}

func checkStrictPositive(key string) error {
	if value := cfg.GetInt(key); value < 1 {
		return fmt.Errorf("invalid value for %s (%d)", key, value)
	}

	return nil
}

func checkConfig() error {
	if err := checkStrictPositive("agent.flowtable_expire"); err != nil {
		return err
	}

	if err := checkStrictPositive("agent.flowtable_update"); err != nil {
		return err
	}

	if err := checkStrictPositive("analyzer.flowtable_expire"); err != nil {
		return err
	}

	if err := checkStrictPositive("analyzer.flowtable_update"); err != nil {
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

func InitConfig(backend string, path string) error {
	if path == "" {
		return fmt.Errorf("Empty configuration path")
	}

	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	if ext == "" || !checkViperSupportedExts(ext) {
		ext = "yaml"
	}
	cfg.SetConfigType(ext)

	switch backend {
	case "file":
		configFile, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := cfg.ReadConfig(configFile); err != nil {
			return err
		}
	case "etcd":
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

func GetConfig() *viper.Viper {
	return cfg
}

func SetDefault(key string, value interface{}) {
	cfg.SetDefault(key, value)
}

func GetHostPortAttributes(s string, p string) (string, int, error) {
	key := s + "." + p
	listen := strings.Split(GetConfig().GetString(key), ":")

	addr := "127.0.0.1"

	switch len(listen) {
	case 1:
		port, err := strconv.Atoi(listen[0])
		if err != nil {
			return "", 0, err
		}

		return addr, port, nil
	case 2:
		port, err := strconv.Atoi(listen[1])
		if err != nil {
			return "", 0, err
		}

		return listen[0], port, nil
	default:
		return "", 0, errors.New(fmt.Sprintf("Malformed listen parameter %s in section %s", s, p))
	}
}

func GetAnalyzerClientAddr() (string, int, error) {
	analyzers := GetConfig().GetStringSlice("agent.analyzers")
	// TODO(safchain) HA Connection ???
	if len(analyzers) > 0 {
		addr := strings.Split(analyzers[0], ":")[0]
		port, err := strconv.Atoi(strings.Split(analyzers[0], ":")[1])
		if err != nil {
			return "", 0, err
		}

		return addr, port, nil
	}
	return "", 0, nil
}
