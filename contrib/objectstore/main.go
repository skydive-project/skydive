/*
 * Copyright (C) 2019 IBM, Inc.
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

package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber"
	"github.com/skydive-project/skydive/logging"
)

const defaultConfigurationFile = "/etc/skydive/skydive-objectstore.yml"

func main() {
	if err := config.InitConfig("file", []string{defaultConfigurationFile}); err != nil {
		logging.GetLogger().Errorf("Failed to initialize config: %s", err.Error())
		os.Exit(1)
	}

	if err := config.InitLogging(); err != nil {
		logging.GetLogger().Errorf("Failed to initialize logging system: %s", err.Error())
		os.Exit(1)
	}

	s, err := subscriber.NewSubscriberFromConfig(config.GetConfig())
	if err != nil {
		logging.GetLogger().Errorf(err.Error())
		os.Exit(1)
	}

	s.Start()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	s.Stop()
}
