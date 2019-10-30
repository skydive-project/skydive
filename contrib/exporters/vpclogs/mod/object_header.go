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

package mod

import (
	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/logging"
)

const version = "0.0.1"

type objectHeaderVpc struct {
	keyValues map[string]string
}

// AddObjHeader inserts the object global paramateres and wraps the flows
func (h *objectHeaderVpc) AddObjHeader(flows []interface{}, startTime string, endTime string) interface{} {
	var augmentedObject map[string]interface{}
	augmentedObject = make(map[string]interface{})
	for key, value := range h.keyValues {
		augmentedObject[key] = value
	}
	augmentedObject["version"] = version
	augmentedObject["capture_start_time"] = startTime
	augmentedObject["capture_end_time"] = endTime
	augmentedObject["number_of_flow_logs"] = len(flows)
	augmentedObject["flow_logs"] = flows
	// TBD can we do better with state?
	augmentedObject["state"] = "skip data"

	return augmentedObject
}

func NewObjHeaderVpc(cfg *viper.Viper) (interface{}, error) {
	keyValues := cfg.GetStringMapString("pipeline.objheader.vpclogs")
	logging.GetLogger().Infof("Defined object header fields:")
	for key, value := range keyValues {
		logging.GetLogger().Infof("          %s: %s", key, value)
	}
	return &objectHeaderVpc{
		keyValues: keyValues,
	}, nil
}
