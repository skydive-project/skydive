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

package client

import (
	"encoding/json"
	"fmt"
	"os"

	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/cobra"
)

var (
	AuthenticationOpts shttp.AuthenticationOpts
)

func printJSON(obj interface{}) {
	s, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		logging.GetLogger().Errorf(err.Error())
		os.Exit(1)
	}
	fmt.Println(string(s))
}

func setFromFlag(cmd *cobra.Command, flag string, value *string) {
	if flag := cmd.LocalFlags().Lookup(flag); flag.Changed {
		*value = flag.Value.String()
	}
}
