/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"fmt"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	DefaultConfigurationFile = "/etc/skydive/skydive.yml"
)

// LoadConfiguration from a configuration file
// If no configuration file are given, try to load the default configuration file /etc/skydive/skydive.yml
func LoadConfiguration(cfgBackend string, cfgFiles []string) error {
	if len(cfgFiles) == 0 {
		config.InitConfig(cfgBackend, []string{DefaultConfigurationFile})
		if err := logging.InitLogging(); err != nil {
			return fmt.Errorf("Failed to initialize logging system: %s", err.Error())
		}
		return nil
	}

	if err := config.InitConfig(cfgBackend, cfgFiles); err != nil {
		return fmt.Errorf("Failed to initialize config: %s", err.Error())
	}

	if err := logging.InitLogging(); err != nil {
		return fmt.Errorf("Failed to initialize logging system: %s", err.Error())
	}

	return nil
}
