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

	"github.com/abbot/go-http-auth"
	"github.com/gorilla/context"
)

// NoAuthenticationBackend describes an authenticate backed that
// allows everyone to do anything
type NoAuthenticationBackend struct {
}

// Name returns the name of the backend
func (n *NoAuthenticationBackend) Name() string {
	return "noauth"
}

// DefaultUserRole returns the name of the backend
func (n *NoAuthenticationBackend) DefaultUserRole(user string) string {
	return DefaultUserRole
}

// SetDefaultUserRole defines the default user role
func (n *NoAuthenticationBackend) SetDefaultUserRole(role string) {
}

// Authenticate the user and its password
func (n *NoAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	return "", nil
}

// Wrap an HTTP handler with no authentication backend
func (n *NoAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ar := &auth.AuthenticatedRequest{Request: *r, Username: "admin"}
		copyRequestVars(r, &ar.Request)
		wrapped(w, ar)
		context.Clear(&ar.Request)
	}
}

// NewNoAuthenticationBackend returns a new authentication backend that allows
// everyone to do anything
func NewNoAuthenticationBackend() *NoAuthenticationBackend {
	return &NoAuthenticationBackend{}
}

// NoAuthenticationWrap wraps a handler with no authentication
func NoAuthenticationWrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return NewNoAuthenticationBackend().Wrap(wrapped)
}
