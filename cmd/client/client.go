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

	"github.com/redhat-cip/skydive/logging"
	"github.com/spf13/cobra"
)

var (
	clientUsername string
	clientPassword string
)

var Client = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
}

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

func init() {
	Client.PersistentFlags().StringVarP(&clientUsername, "username", "", os.Getenv("SKYDIVE_USERNAME"), "username auth parameter")
	Client.PersistentFlags().StringVarP(&clientPassword, "password", "", os.Getenv("SKYDIVE_PASSWORD"), "password auth parameter")

	Client.AddCommand(AlertCmd)
	Client.AddCommand(CaptureCmd)
}
