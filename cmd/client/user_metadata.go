/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"os"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"

	"github.com/spf13/cobra"
)

var (
	key   string
	value string
)

//UserMetadataCmd skydive user-metadata root command
var UserMetadataCmd = &cobra.Command{
	Use:          "user-metadata",
	Short:        "Manage user metadata",
	Long:         "Manage user metadata",
	SilenceUsage: false,
}

// UserMetadataCreate skydive user-metadata add command
var UserMetadataCreate = &cobra.Command{
	Use:          "create",
	Short:        "create user metadata",
	Long:         "create user metadata",
	SilenceUsage: false,
	PreRun: func(cmd *cobra.Command, args []string) {
		if gremlinQuery == "" || key == "" || value == "" {
			logging.GetLogger().Error("All parameters are mantatory")
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Critical(err)
			os.Exit(1)
		}
		metadata := api.NewUserMetadata(gremlinQuery, key, value)

		if err := validator.Validate(metadata); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		if err := client.Create("usermetadata", &metadata); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}
		printJSON(metadata)
	},
}

// UserMetadataDelete skydive user-metadata delete command
var UserMetadataDelete = &cobra.Command{
	Use:          "delete",
	Short:        "delete user metadata",
	Long:         "delete user metadata",
	SilenceUsage: false,
	PreRun: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Usage()
			os.Exit(1)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Critical(err)
			os.Exit(1)
		}

		for _, id := range args {
			if err := client.Delete("usermetadata", id); err != nil {
				logging.GetLogger().Error(err)
			}
		}
	},
}

// UserMetadataList skydive user-metadata list command
var UserMetadataList = &cobra.Command{
	Use:          "list",
	Short:        "List user metadata",
	Long:         "List user metadata",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		var metadata map[string]api.UserMetadata
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Critical(err)
			os.Exit(1)
		}

		if err := client.List("usermetadata", &metadata); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}
		printJSON(metadata)
	},
}

func addUserMetadataFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&gremlinQuery, "gremlin", "", "", "Node gremlin expression")
	cmd.Flags().StringVarP(&key, "key", "", "", "Metadata key")
	cmd.Flags().StringVarP(&value, "value", "", "", "Metadata value")
}

func init() {
	UserMetadataCmd.AddCommand(UserMetadataCreate)
	UserMetadataCmd.AddCommand(UserMetadataDelete)
	UserMetadataCmd.AddCommand(UserMetadataList)

	addUserMetadataFlags(UserMetadataCreate)
}
