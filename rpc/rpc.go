/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package rpc

import (
	"net/http"

	"github.com/gorilla/mux"
)

type PathPrefix string

type Route struct {
	Name        string
	Method      string
	Path        interface{}
	HandlerFunc http.HandlerFunc
}

func RegisterRoutes(router *mux.Router, routes []Route) {
	for _, route := range routes {
		r := router.
			Methods(route.Method).
			Name(route.Name).
			Handler(route.HandlerFunc)
		switch p := route.Path.(type) {
		case string:
			r.Path(p)
		case PathPrefix:
			r.PathPrefix(string(p))
		}
	}
}
