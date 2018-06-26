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
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/http"

	"github.com/spf13/cobra"
)

var alertOpts = struct {
	Name        string `flag:"name,alert name"`
	Description string `flag:"description,description of the alert"`
	Trigger     string `flag:"trigger,event that triggers the alert evaluation"`
	Expression  string `flag:"expression,Gremlin of JavaScript expression evaluated to trigger the alarm"`
	Action      string `flag:"action,can be either an empty string, or a URL (use 'file://' for local scripts)"`
}{
	Trigger: "graph",
}

var alertCmd = crudRootCommand{
	Resource: "alert",

	Get: &getHandler{
		Run: func(client *http.CrudClient, id string) (types.Resource, error) {
			var alert types.Alert
			err := client.Get("alert", id, &alert)
			return &alert, err
		},
	},

	List: &listHandler{
		Run: func(client *http.CrudClient) (map[string]types.Resource, error) {
			var alerts map[string]types.Alert
			if err := client.List("alert", &alerts); err != nil {
				return nil, err
			}
			resources := make(map[string]types.Resource, len(alerts))
			for _, alert := range alerts {
				resources[alert.ID()] = &alert
			}
			return resources, nil
		},
	},

	Create: &createHandler{
		Flags: func(crud *crudRootCommand, c *cobra.Command) {
			crud.setFlags(c, &alertOpts)
		},

		Run: func(client *http.CrudClient) (types.Resource, error) {
			alert := types.NewAlert()
			alert.Name = alertOpts.Name
			alert.Description = alertOpts.Description
			alert.Expression = alertOpts.Expression
			alert.Trigger = alertOpts.Trigger
			alert.Action = alertOpts.Action
			return alert, nil
		},
	},
}
