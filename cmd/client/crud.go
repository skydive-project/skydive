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
	"reflect"
	"strings"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/validator"
	"github.com/spf13/cobra"
)

type createHandler struct {
	Flags  func(root *crudRootCommand, c *cobra.Command)
	PreRun func() error
	Run    func(c *http.CrudClient) (types.Resource, error)
}

type listHandler struct {
	Flags func(root *crudRootCommand, c *cobra.Command)
	Run   func(c *http.CrudClient) (map[string]types.Resource, error)
}

type getHandler struct {
	Flags func(root *crudRootCommand, c *cobra.Command)
	Run   func(c *http.CrudClient, id string) (types.Resource, error)
}

type deleteHandler struct {
	Flags func(root *crudRootCommand, c *cobra.Command)
	Run   func(c *http.CrudClient, id string) error
}

type crudRootCommand struct {
	Resource string
	Command  string
	Name     string
	NoPlural bool
	Create   *createHandler
	List     *listHandler
	Get      *getHandler
	Delete   *deleteHandler
}

func (cmd *crudRootCommand) setFlags(c *cobra.Command, x interface{}) {
	xv := reflect.ValueOf(x).Elem()
	t := xv.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tags := field.Tag.Get("flag")
		props := strings.SplitN(tags, ",", 2)
		if len(props) == 2 {
			addr := xv.Field(i).Addr().Interface()
			switch ptr := addr.(type) {
			case *int:
				c.Flags().IntVarP(ptr, props[0], "", int(xv.Field(i).Int()), props[1])
			case *string:
				c.Flags().StringVarP(ptr, props[0], "", xv.Field(i).String(), props[1])
			case *bool:
				c.Flags().BoolVarP(ptr, props[0], "", xv.Field(i).Bool(), props[1])
			}
		}
	}
}

func (cmd *crudRootCommand) createClient() *http.CrudClient {
	client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
	if err != nil {
		logging.GetLogger().Error(err.Error())
		os.Exit(1)
	}
	return client
}

func (cmd *crudRootCommand) Cmd() *cobra.Command {
	resource := cmd.Resource
	use := cmd.Command
	if use == "" {
		use = resource
	}
	name := cmd.Name
	if name == "" {
		name = resource
	}
	plural := name
	if !cmd.NoPlural {
		plural += "s"
	}
	rootCmd := &cobra.Command{
		Use:          use,
		Short:        "Manage " + plural,
		Long:         "Manage " + plural,
		SilenceUsage: false,
	}

	if cmd.List != nil {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "list",
			Short: "List " + plural,
			Long:  "List " + plural,
			Run: func(c *cobra.Command, args []string) {
				resources, err := cmd.List.Run(cmd.createClient())
				if err != nil {
					logging.GetLogger().Error(err)
					os.Exit(1)
				}
				printJSON(resources)
			},
		})
	}

	if cmd.Create != nil {
		createCommand := &cobra.Command{
			Use:   "create",
			Short: "Create " + name,
			Long:  "Create " + name,
			PreRun: func(c *cobra.Command, args []string) {
				if cmd.Create.PreRun != nil {
					if err := cmd.Create.PreRun(); err != nil {
						logging.GetLogger().Error(err)
						os.Exit(1)
					}
				}
			},
			Run: func(c *cobra.Command, args []string) {
				client := cmd.createClient()
				resource, err := cmd.Create.Run(client)
				if err != nil {
					logging.GetLogger().Error(err)
					os.Exit(1)
				}

				if err := validator.Validate(resource); err != nil {
					logging.GetLogger().Error(err)
					os.Exit(1)
				}

				if err := client.Create(cmd.Resource, resource); err != nil {
					logging.GetLogger().Error(err)
					os.Exit(1)
				}
			},
		}

		if cmd.Create.Flags != nil {
			cmd.Create.Flags(cmd, createCommand)
		}

		rootCmd.AddCommand(createCommand)
	}

	if cmd.Get != nil {
		rootCmd.AddCommand(&cobra.Command{
			Use:   "get",
			Short: "Get " + name,
			Long:  "Get " + name,
			PreRun: func(c *cobra.Command, args []string) {
				if len(args) == 0 {
					c.Usage()
					os.Exit(1)
				}
			},
			Run: func(c *cobra.Command, args []string) {
				resource, err := cmd.Get.Run(cmd.createClient(), args[0])
				if err != nil {
					logging.GetLogger().Error(err)
					os.Exit(1)
				}
				printJSON(resource)
			},
		})
	}

	rootCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete " + name,
		Long:  "Delete " + name,
		Run: func(c *cobra.Command, args []string) {
			client := cmd.createClient()
			for _, id := range args {
				if err := client.Delete(resource, id); err != nil {
					logging.GetLogger().Error(err)
				}
			}
		},
	})

	return rootCmd
}
