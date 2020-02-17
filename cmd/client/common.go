/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package client

import (
	"encoding/json"
	"fmt"
	"os"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// AuthenticationOpts Authentication options
var (
	AuthenticationOpts shttp.AuthenticationOpts
)

func printJSON(obj interface{}) {
	s, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}
	fmt.Println(string(s))
}
