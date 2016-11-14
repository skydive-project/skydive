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
	"encoding/base64"
	"net/http"
	"os"

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/context"
	"github.com/skydive-project/skydive/config"
)

const (
	basicAuthRealm string = "Skydive Authentication"
)

type BasicAuthenticationBackend struct {
	*auth.BasicAuth
}

func (b *BasicAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	request := &http.Request{Header: make(map[string][]string)}
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	request.Header.Set("Authorization", "Basic "+creds)

	if username := b.CheckAuth(request); username == "" {
		return "", WrongCredentials
	}

	return creds, nil
}

func (b *BasicAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("authtok")
		if err == nil {
			r.Header.Set("Authorization", "Basic "+cookie.Value)
		}

		if username := b.CheckAuth(r); username == "" {
			unauthorized(w, r)
		} else {
			ar := &auth.AuthenticatedRequest{Request: *r, Username: username}
			copyRequestVars(r, &ar.Request)
			wrapped(w, ar)
			context.Clear(&ar.Request)
		}
	}
}

func NewBasicAuthenticationBackend(file string) (*BasicAuthenticationBackend, error) {
	if _, err := os.Stat(file); err != nil {
		return nil, err
	}

	// TODO(safchain) add more providers
	h := auth.HtpasswdFileProvider(file)
	return &BasicAuthenticationBackend{
		auth.NewBasicAuthenticator(basicAuthRealm, h),
	}, nil
}

func NewBasicAuthenticationBackendFromConfig() (*BasicAuthenticationBackend, error) {
	f := config.GetConfig().GetString("auth.basic.file")
	return NewBasicAuthenticationBackend(f)
}
