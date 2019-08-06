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

package core

import (
	"os"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

// Main entry point for exporters
func Main(defaultCfgFile string) {
	if len(os.Args) > 1 {
		defaultCfgFile = os.Args[1]
	}

	if err := config.InitConfig("file", []string{defaultCfgFile}); err != nil {
		logging.GetLogger().Errorf("Failed to initialize config: %s", err)
		os.Exit(1)
	}

	if err := config.InitLogging(); err != nil {
		logging.GetLogger().Errorf("Failed to initialize logging system: %s", err)
		os.Exit(1)
	}

	pipeline, err := NewPipeline(config.GetConfig().Viper)
	if err != nil {
		logging.GetLogger().Errorf("Failed to initialize pipeline: %s", err)
		os.Exit(1)
	}

	speaker, err := NewSubscriber(pipeline, config.GetConfig().Viper)
	if err != nil {
		logging.GetLogger().Errorf("Failed to initialize subscriber: %s", err)
		os.Exit(1)
	}

	SubscriberRun(speaker)
}
