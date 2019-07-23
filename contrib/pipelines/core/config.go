/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package core

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// NewConfig returns a new viper config with defaults
func NewConfig() *viper.Viper {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	cfg := viper.New()

	cfg.SetDefault("host_id", host)

	cfg.SetDefault("logging.backends", []string{"stderr"})
	cfg.SetDefault("logging.color", true)
	cfg.SetDefault("logging.encoder", "")
	cfg.SetDefault("logging.level", "INFO")

	replacer := strings.NewReplacer(".", "_", "-", "_")
	cfg.SetEnvPrefix("SKYDIVE")
	cfg.SetEnvKeyReplacer(replacer)
	cfg.AutomaticEnv()
	cfg.SetTypeByDefaultValue(true)

	return cfg
}
