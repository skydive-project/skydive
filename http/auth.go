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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/abbot/go-http-auth"
	"github.com/skydive-project/skydive/config"
)

var (
	WrongCredentials error = errors.New("Wrong credentials")
)

type AuthenticationOpts struct {
	Username string
	Password string
}

type AuthenticationClient struct {
	authOptions   *AuthenticationOpts
	authenticated bool
	Addr          string
	Port          int
	AuthToken     string
}

func (c *AuthenticationClient) getPrefix() string {
	return fmt.Sprintf("http://%s:%d", c.Addr, c.Port)
}

func (c *AuthenticationClient) Authenticated() bool {
	return c.authenticated
}

func (c *AuthenticationClient) SetHeaders(headers http.Header) {
	if c.authenticated && c.AuthToken != "" {
		headers.Set("Cookie", c.Cookie().String())
	}
}

func (c *AuthenticationClient) Cookie() *http.Cookie {
	return &http.Cookie{Name: "authtok", Value: c.AuthToken}
}

func (c *AuthenticationClient) Authenticate() error {
	values := url.Values{"username": {c.authOptions.Username}, "password": {c.authOptions.Password}}

	req, err := http.NewRequest("POST", c.getPrefix()+"/login", strings.NewReader(values.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("Authentication failed: %s", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusFound {
		return fmt.Errorf("Authentication failed: returned code %d", resp.StatusCode)
	}

	c.authenticated = true

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "authtok" {
			c.AuthToken = cookie.Value
			break
		}
	}

	return nil
}

func NewAuthenticationClient(addr string, port int, authOptions *AuthenticationOpts) *AuthenticationClient {
	return &AuthenticationClient{
		Addr:        addr,
		Port:        port,
		authOptions: authOptions,
	}
}

type AuthenticationBackend interface {
	Authenticate(username string, password string) (string, error)
	Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc
}

func NewAuthenticationBackendFromConfig() (AuthenticationBackend, error) {
	t := config.GetConfig().GetString("auth.type")

	switch t {
	case "basic":
		return NewBasicAuthenticationBackendFromConfig()
	case "keystone":
		return NewKeystoneAuthenticationBackendFromConfig(), nil
	default:
		return NewNoAuthenticationBackend(), nil
	}
}
