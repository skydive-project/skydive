/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package http

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/context"

	"github.com/skydive-project/skydive/graffiti/rbac"
)

var (
	// ErrWrongCredentials error wrong credentials
	ErrWrongCredentials = errors.New("Wrong credentials")
)

const (
	// DefaultUserRole is the default role to assign to a user
	DefaultUserRole = "admin"
	tokenName       = "authtoken"
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

func setPermissionsCookie(w http.ResponseWriter, permissions []rbac.Permission) {
	jsonPerms, _ := json.Marshal(permissions)
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
func Authenticate(backend AuthenticationBackend, w http.ResponseWriter, username, password string) (string, []rbac.Permission, error) {
	token, err := backend.Authenticate(username, password)
	if err != nil {
		return "", nil, err
	}

	if roles := rbac.GetUserRoles(username); len(roles) == 0 {
		rbac.AddRoleForUser(username, backend.DefaultUserRole(username))
	}

	if token != "" {
		http.SetCookie(w, AuthCookie(token, "/"))
	}

	permissions := rbac.GetPermissionsForUser(username)
	setPermissionsCookie(w, permissions)

	return token, permissions, nil
}

func tokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie(tokenName); err == nil {
		return cookie.Value
	}

	if authorization := r.Header.Get("Authorization"); authorization != "" {
		return authorization
	}

	if token := r.Header.Get("X-Auth-Token"); token != "" {
		return token
	}

	r.ParseForm()
	if tokens := r.Form["x-auth-token"]; len(tokens) != 0 {
		return tokens[0]
	}

	return ""
}
