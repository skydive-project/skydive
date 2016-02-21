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
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

var cfg *viper.Viper

func init() {
	cfg = viper.New()
	cfg.SetDefault("agent.listen", "127.0.0.1:8081")
	cfg.SetDefault("agent.flowtable_expire", 10)
	cfg.SetDefault("ovs.ovsdb", "127.0.0.1:6400")
	cfg.SetDefault("graph.backend", "memory")
	cfg.SetDefault("graph.gremlin", "127.0.0.1:8182")
	cfg.SetDefault("sflow.listen", "127.0.0.1:6345")
	cfg.SetDefault("analyzer.listen", "127.0.0.1:8082")
	cfg.SetDefault("analyzer.flowtable_expire", 10)
	cfg.SetDefault("storage.elasticsearch", "127.0.0.1:9200")
	cfg.SetDefault("ws_pong_timeout", 5)
}

func checkStrictPositive(key string) error {
	if value := cfg.GetInt(key); value < 1 {
		return fmt.Errorf("invalid value for %s (%d)", value)
	}

	return nil
}

func checkConfig() error {
	if err := checkStrictPositive("agent.flowtable_expire"); err != nil {
		return err
	}

	if err := checkStrictPositive("analyzer.flowtable_expire"); err != nil {
		return err
	}

	return nil
}

func InitConfigFromFile(filename string) error {
	configFile, err := os.Open(filename)
	if err != nil {
		return err
	}

	cfg.SetConfigType("yaml")
	if err := cfg.ReadConfig(configFile); err != nil {
		return err
	}

	if err := checkConfig(); err != nil {
		return nil
	}

	return nil
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

	switch l := len(listen); {
	case l == 1:
		port, err := strconv.Atoi(listen[0])
		if err != nil {
			return "", 0, err
		}

		return addr, port, nil
	case l == 2:
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
