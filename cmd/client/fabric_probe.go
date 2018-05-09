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

package client

import (
	"os"

	"github.com/skydive-project/skydive/api/client"
	api "github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/logging"

	"github.com/spf13/cobra"
)

var (
	elementType string
	subType     string
	name        string
	metadata    string
	parentID    string
)

// FabricProbeCmd skydive fabric root command
var FabricProbeCmd = &cobra.Command{
	Use:          "fabric",
	Short:        "fabric",
	Long:         "fabric",
	SilenceUsage: false,
}

// FabricProbeCreate describes the command to create a fabric node
var FabricProbeCreate = &cobra.Command{
	Use:          "create",
	Short:        "create fabric probe",
	Long:         "create fabric probe",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
		if err != nil {
			logging.GetLogger().Critical(err.Error())
			os.Exit(1)
		}

		fp := &api.FabricProbe{
			SrcNode:  srcNode,
			DstNode:  dstNode,
			Type:     elementType,
			SubType:  subType,
			Name:     name,
			Metadata: metadata,
			ParentID: parentID,
		}

		if err := client.Create("fabricprobe", &fp); err != nil {
			logging.GetLogger().Error(err.Error())
			os.Exit(1)
		}

		printJSON(fp)
	},
}

func addCreateFabricFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&srcNode, "src", "", "", "source node of the edge")
	cmd.Flags().StringVarP(&dstNode, "dst", "", "", "destination node of the edge")
	cmd.Flags().StringVarP(&elementType, "type", "", "", "element type: node, edge")
	cmd.Flags().StringVarP(&subType, "subType", "", "", "sub type: host/port or layer2/ownership")
	cmd.Flags().StringVarP(&name, "name", "", "", "node name")
	cmd.Flags().StringVarP(&metadata, "metadata", "", "", "metadata")
	cmd.Flags().StringVarP(&parentID, "parentID", "", "", "parentID")
}

func init() {
	FabricProbeCmd.AddCommand(FabricProbeCreate)

	addCreateFabricFlags(FabricProbeCreate)
}
