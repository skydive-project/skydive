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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gorilla/context"

	"github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/rbac"
)

var (
	// ErrWrongCredentials error wrong credentials
	ErrWrongCredentials = errors.New("Wrong credentials")
)

const (
	// DefaultUserRole is the default role to assign to a user
	DefaultUserRole = "admin"
	tokenName       = "authtok"
)

// AuthenticationOpts describes the elements used by a client to authenticate
// to an HTTP server. It can be either a username/password couple or a token
type AuthenticationOpts struct {
	Username string
	Password string
	Token    string
	Cookie   map[string]string
}

// AuthCookie returns a authentication cookie
func AuthCookie(token, path string) *http.Cookie {
	return &http.Cookie{Name: tokenName, Value: token, Path: path}
}

// SetAuthHeaders apply all the cookie used for authentication to the header
func SetAuthHeaders(headers *http.Header, authOpts *AuthenticationOpts) {
	cookies := []*http.Cookie{}
	if authOpts.Token != "" {
		cookies = append(cookies, AuthCookie(authOpts.Token, ""))
	} else if authOpts.Username != "" {
		basic := base64.StdEncoding.EncodeToString([]byte(authOpts.Username + ":" + authOpts.Password))
		headers.Set("Authorization", "Basic "+basic)
	}

	// cookie that comes from the config, can be used with proxies
	for name, value := range authOpts.Cookie {
		cookies = append(cookies, &http.Cookie{Name: name, Value: value})
	}

	var b bytes.Buffer
	for _, cookie := range cookies {
		b.WriteString(cookie.String())
		b.WriteString("; ")
	}
	headers.Set("Cookie", b.String())
}

// AuthenticationBackend is the interface of a authentication backend
type AuthenticationBackend interface {
	Name() string
	DefaultUserRole(user string) string
	SetDefaultUserRole(role string)
	Authenticate(username string, password string) (string, error)
	Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc
}

func setPermissionsCookie(w http.ResponseWriter, username string) {
	jsonPerms, _ := json.Marshal(rbac.GetPermissionsForUser(username))
	http.SetCookie(w, &http.Cookie{
		Name:  "permissions",
		Value: base64.StdEncoding.EncodeToString([]byte(jsonPerms)),
		Path:  "/",
	})
}

func authCallWrapped(w http.ResponseWriter, r *http.Request, username string, wrapped auth.AuthenticatedHandlerFunc) {
	ar := &auth.AuthenticatedRequest{Request: *r, Username: username}
	copyRequestVars(r, &ar.Request)
	wrapped(w, ar)
	context.Clear(&ar.Request)
}

// Authenticate checks a couple of username and password against an authentication backend.
// If it succeeds, it set a token as a HTTP cookie. It then retrieves the roles for the
// authenticated user from the backend.
func Authenticate(backend AuthenticationBackend, w http.ResponseWriter, username, password string) (string, error) {
	token, err := backend.Authenticate(username, password)
	if err != nil {
		return "", err
	}

	if roles := rbac.GetUserRoles(username); len(roles) == 0 {
		rbac.AddRoleForUser(username, backend.DefaultUserRole(username))
	}

	if token != "" {
		http.SetCookie(w, AuthCookie(token, "/"))
	}

	setPermissionsCookie(w, username)

	return token, nil
}

// Authenticate uses request and the given backend to authenticate
func authenticateWithHeaders(backend AuthenticationBackend, w http.ResponseWriter, r *http.Request) (string, error) {
	// first try to get an already retrieve auth token through cookie
	cookie, err := r.Cookie(tokenName)
	if err == nil {
		http.SetCookie(w, AuthCookie(cookie.Value, "/"))
		return cookie.Value, nil
	}

	authorization := r.Header.Get("Authorization")
	if authorization == "" {
		return "", nil
	}

	s := strings.SplitN(authorization, " ", 2)
	if len(s) != 2 || s[0] != "Basic" {
		return "", ErrWrongCredentials
	}

	b, err := base64.StdEncoding.DecodeString(s[1])
	if err != nil {
		return "", ErrWrongCredentials
	}
	pair := strings.SplitN(string(b), ":", 2)
	if len(pair) != 2 {
		return "", ErrWrongCredentials
	}
	username, password := pair[0], pair[1]

	return Authenticate(backend, w, username, password)
}
