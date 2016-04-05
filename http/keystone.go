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
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/identity/v2/tokens"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
)

type KeystoneAuthenticationBackend struct {
	AuthURL string
	Tenant  string
}

func (b *KeystoneAuthenticationBackend) CheckUser(r *http.Request) (string, error) {
	cookie, err := r.Cookie("authtok")
	if err != nil {
		return "", WrongCredentials
	}

	tokenID := cookie.Value
	if tokenID == "" {
		return "", WrongCredentials
	}

	provider, err := openstack.NewClient(b.AuthURL)
	if err != nil {
		return "", err
	}

	provider.TokenID = cookie.Value
	client := &gophercloud.ServiceClient{ProviderClient: provider, Endpoint: b.AuthURL}
	result := tokens.Get(client, tokenID)

	user, err := result.ExtractUser()
	if err != nil {
		return "", err
	}

	token, err := result.ExtractToken()
	if err != nil {
		return "", err
	}

	if token.Tenant.Name != b.Tenant {
		return "", WrongCredentials
	}

	isAdmin := false
	for _, role := range user.Roles {
		if role.Name == "admin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		return "", WrongCredentials
	}

	return user.UserName, nil
}

func (b *KeystoneAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: b.AuthURL,
		Username:         username,
		Password:         password,
		TenantName:       b.Tenant,
	}

	provider, err := openstack.NewClient(b.AuthURL)
	if err != nil {
		return "", err
	}

	if err := openstack.Authenticate(provider, opts); err != nil {
		return "", err
	}

	return provider.TokenID, nil
}

func (b *KeystoneAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if username, err := b.CheckUser(r); username == "" {
			if err != nil {
				logging.GetLogger().Warningf("Failed to check token: %s", err.Error())
			}
			unauthorized(w, r)
		} else {
			ar := &auth.AuthenticatedRequest{Request: *r, Username: username}
			wrapped(w, ar)
		}
	}
}

func NewKeystoneBackend(authURL string, tenant string) *KeystoneAuthenticationBackend {
	if !strings.HasSuffix(authURL, "/") {
		authURL += "/"
	}

	return &KeystoneAuthenticationBackend{
		AuthURL: authURL,
		Tenant:  tenant,
	}
}

func NewKeystoneAuthenticationBackendFromConfig() *KeystoneAuthenticationBackend {
	authURL := config.GetConfig().GetString("openstack.auth_url")
	tenant := config.GetConfig().GetString("openstack.tenant_name")

	return NewKeystoneBackend(authURL, tenant)
}
