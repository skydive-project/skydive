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
	"errors"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/http"
	"github.com/spf13/cobra"
)

var userMetadataOpts = struct {
	GremlinQuery string `flag:"gremlin,node gremlin expression"`
	Key          string `flag:"key,Metadata key"`
	Value        string `flag:"value,Metadata value"`
}{}

var userMetadataCmd = crudRootCommand{
	Resource: "usermetadata",
	Command:  "user-metadata",
	Name:     "user metadata",
	NoPlural: true,

	Get: &getHandler{
		Run: func(client *http.CrudClient, id string) (types.Resource, error) {
			var metadata types.UserMetadata
			err := client.Get("usermetadata", id, &metadata)
			return &metadata, err
		},
	},

	List: &listHandler{
		Run: func(client *http.CrudClient) (map[string]types.Resource, error) {
			var metadata map[string]types.UserMetadata
			if err := client.List("usermetadata", &metadata); err != nil {
				return nil, err
			}
			resources := make(map[string]types.Resource, len(metadata))
			for _, data := range metadata {
				resources[data.ID()] = &data
			}
			return resources, nil
		},
	},

	Create: &createHandler{
		Flags: func(crud *crudRootCommand, c *cobra.Command) {
			crud.setFlags(c, &userMetadataOpts)
		},

		PreRun: func() error {
			if userMetadataOpts.GremlinQuery == "" || userMetadataOpts.Key == "" || userMetadataOpts.Value == "" {
				return errors.New("All parameters are mandatory")
			}
			return nil
		},

		Run: func(client *http.CrudClient) (types.Resource, error) {
			return types.NewUserMetadata(userMetadataOpts.GremlinQuery, userMetadataOpts.Key, userMetadataOpts.Value), nil
		},
	},
}
