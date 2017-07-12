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

package logging

import (
	"fmt"
	"os"
	"strings"

	"github.com/op/go-logging"
	"github.com/skydive-project/skydive/config"
)

var logger *logging.Logger

func initLogger() (_ *logging.Logger, err error) {
	cfg := config.GetConfig()
	id := cfg.GetString("host_id") + ":" + cfg.GetString("logging.id")

	format := cfg.GetString("logging.format")
	format = strings.Replace(format, "%{id}", id, -1)

	level, err := logging.LogLevel(cfg.GetString("logging.level"))
	if err != nil {
		return nil, err
	}

	var backends []logging.Backend
	var backend logging.Backend
	for _, name := range cfg.GetStringSlice("logging.backends") {
		switch name {
		case "file":
			filename := cfg.GetString("logging.file.path")
			file, err := os.Create(filename)
			if err != nil {
				return nil, err
			}
			backend = logging.NewLogBackend(file, "", 0)
		case "syslog":
			backend, err = logging.NewSyslogBackend("")
			if err != nil {
				return nil, err
			}
		case "stderr":
			backend = logging.NewLogBackend(os.Stderr, "", 0)
		default:
			return nil, fmt.Errorf("Invalid logging backend: %s", name)
		}

		backend = logging.NewBackendFormatter(backend, logging.MustStringFormatter(format))
		backends = append(backends, backend)
	}

	backendLevel := logging.MultiLogger(backends...)
	backendLevel.SetLevel(level, "")

	logger, err := logging.GetLogger("")
	if err != nil {
		return nil, err
	}
	logger.SetBackend(backendLevel)

	return logger, nil
}

// GetLogger returns the current logger instance
func GetLogger() (log *logging.Logger) {
	if logger == nil {
		logger, err := logging.GetLogger("")
		if err != nil {
			panic(err)
		}
		return logger
	}
	return logger
}

// InitLogging initialize the logger
func InitLogging() (err error) {
	logger, err = initLogger()
	return err
}
