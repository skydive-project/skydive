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

package http

import (
	"net/http"
	"os"

	auth "github.com/abbot/go-http-auth"
	"github.com/redhat-cip/skydive/config"
)

type AuthRouteHandlerWrapper interface {
	Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc
}

type NoAuthRouteHandlerWrapper struct {
}

func (h *NoAuthRouteHandlerWrapper) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ar := &auth.AuthenticatedRequest{Request: *r, Username: ""}
		wrapped(w, ar)
	}
}

func NewNoAuthRouteHandlerWrapper() *NoAuthRouteHandlerWrapper {
	return &NoAuthRouteHandlerWrapper{}
}

func NewAuthRouteHandlerFromConfig() (AuthRouteHandlerWrapper, error) {
	t := config.GetConfig().GetString("auth.type")

	switch t {
	case "basic":
		f := config.GetConfig().GetString("auth.basic.file")
		if _, err := os.Stat(f); err != nil {
			return nil, err
		}

		// TODO(safchain) add more providers
		h := auth.HtpasswdFileProvider(f)
		return auth.NewBasicAuthenticator("Skydive Authentication", h), nil
	default:
		return NewNoAuthRouteHandlerWrapper(), nil
	}
}
