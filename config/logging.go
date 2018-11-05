/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"os"

	"github.com/skydive-project/skydive/logging"
)

// InitLogging set up logging based on the section "logging" of
// the configuration file
func InitLogging() error {
	color := GetBool("logging.color")
	id := GetString("host_id") + ":" + GetString("logging.id")
	defaultEncoder := cfg.GetString("logging.encoder")
	defaultLogLevel := cfg.GetString("logging.level")

	var err error
	var backend logging.Backend
	var loggers []*logging.LoggerConfig
	for _, name := range cfg.GetStringSlice("logging.backends") {
		switch name {
		case "file":
			filename := cfg.GetString("logging.file.path")
			backend, err = logging.NewFileBackend(filename)
			if err != nil {
				return err
			}
		case "syslog":
			syslogTag := cfg.GetString("logging.syslog.tag")
			backend, err = logging.NewSyslogBackend(syslogTag)
			if err != nil {
				return err
			}
		case "stderr":
			backend = logging.NewStdioBackend(os.Stderr)
		case "stdout":
			backend = logging.NewStdioBackend(os.Stdout)
		default:
			return fmt.Errorf("Invalid logging backend: %s", name)
		}

		prefix := "logging." + name
		encoder := defaultEncoder
		logLevel := defaultLogLevel
		if e := cfg.GetString(prefix + ".encoder"); e != "" {
			encoder = e
		}
		if l := cfg.GetString(prefix + ".level"); l != "" {
			logLevel = l
		}

		loggers = append(loggers, logging.NewLoggerConfig(backend, logLevel, encoder))
	}

	return logging.InitLogging(id, color, loggers)
}
