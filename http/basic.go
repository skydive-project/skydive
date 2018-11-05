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

	"github.com/abbot/go-http-auth"
)

const (
	basicAuthRealm string = "Skydive Authentication"
)

// BasicAuthenticationBackend implements HTTP BasicAuth authentication
type BasicAuthenticationBackend struct {
	*auth.BasicAuth
	name string
	role string
}

// Name returns the name of the backend
func (b *BasicAuthenticationBackend) Name() string {
	return b.name
}

// DefaultUserRole returns the default user role
func (b *BasicAuthenticationBackend) DefaultUserRole(user string) string {
	return b.role
}

// SetDefaultUserRole defines the default user role
func (b *BasicAuthenticationBackend) SetDefaultUserRole(role string) {
	b.role = role
}

// Authenticate the user and its password
func (b *BasicAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	request := &http.Request{Header: make(http.Header)}
	creds := base64.StdEncoding.EncodeToString([]byte(username + ":" + password))
	request.Header.Set("Authorization", "Basic "+creds)

	username = b.CheckAuth(request)
	if username == "" {
		return "", ErrWrongCredentials
	}

	return creds, nil
}

// Wrap an HTTP handler with BasicAuth authentication
func (b *BasicAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, err := authenticateWithHeaders(b, w, r)
		if err != nil {
			Unauthorized(w, r)
			return
		}

		// add "fake" header to let the basic auth library do the authentication
		r.Header.Set("Authorization", "Basic "+token)

		if username := b.CheckAuth(r); username == "" {
			Unauthorized(w, r)
		} else {
			authCallWrapped(w, r, username, wrapped)
		}
	}
}

// NewBasicAuthenticationBackend returns a new BasicAuth authentication backend
func NewBasicAuthenticationBackend(name string, provider auth.SecretProvider, role string) (*BasicAuthenticationBackend, error) {
	return &BasicAuthenticationBackend{
		BasicAuth: auth.NewBasicAuthenticator(basicAuthRealm, provider),
		name:      name,
		role:      role,
	}, nil
}
