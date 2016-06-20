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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	tokens2 "github.com/gophercloud/gophercloud/openstack/identity/v2/tokens"
	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gorilla/context"
	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

type KeystoneAuthenticationBackend struct {
	AuthURL string
	Tenant  string
	Domain  string
}

type User struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
}

func (b *KeystoneAuthenticationBackend) checkUserV2(client *gophercloud.ServiceClient, tokenID string) (string, error) {
	result := tokens2.Get(client, tokenID)

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

func (b *KeystoneAuthenticationBackend) checkUserV3(client *gophercloud.ServiceClient, tokenID string) (string, error) {
	result := tokens3.Get(client, tokenID)

	type Role struct {
		Name string `mapstructure:"name"`
	}

	var response struct {
		Token struct {
			User    User   `mapstructure:"user"`
			Roles   []Role `mapstructure:"roles"`
			Project struct {
				Name   string `mapstructure:"name"`
				Domain struct {
					Name string `mapstructure:"name"`
				} `mapstructure:"domain"`
			}
		} `mapstructure:"token"`
	}
	mapstructure.Decode(result.Body, &response)

	// test that the project is the same as the one provided in the conf file
	if response.Token.Project.Name != b.Tenant || response.Token.Project.Domain.Name != b.Domain {
		return "", WrongCredentials
	}

	// test that the user is the admin of the project
	isAdmin := false
	for _, role := range response.Token.Roles {
		if role.Name == "admin" {
			isAdmin = true
			break
		}
	}

	if !isAdmin {
		return "", WrongCredentials
	}

	return response.Token.User.Name, nil
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

	client := &gophercloud.ServiceClient{
		ProviderClient: provider,
		Endpoint:       b.AuthURL,
	}

	if b.Domain != "" {
		return b.checkUserV3(client, tokenID)
	}

	return b.checkUserV2(client, tokenID)
}

func (b *KeystoneAuthenticationBackend) Authenticate(username string, password string) (string, error) {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: b.AuthURL,
		Username:         username,
		Password:         password,
		TenantName:       b.Tenant,
		DomainName:       b.Domain,
	}

	provider, err := openstack.NewClient(b.AuthURL)
	if err != nil {
		return "", err
	}

	if err := openstack.Authenticate(provider, opts); err != nil {
		logging.GetLogger().Noticef("Keystone authentication error : %s", err)
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
			copyRequestVars(r, &ar.Request)
			wrapped(w, ar)
			context.Clear(&ar.Request)
		}
	}
}

func NewKeystoneBackend(authURL string, tenant string, domain string) *KeystoneAuthenticationBackend {
	if !strings.HasSuffix(authURL, "/") {
		authURL += "/"
	}

	return &KeystoneAuthenticationBackend{
		AuthURL: authURL,
		Tenant:  tenant,
		Domain:  domain,
	}
}

func NewKeystoneAuthenticationBackendFromConfig() *KeystoneAuthenticationBackend {
	authURL := config.GetConfig().GetString("openstack.auth_url")
	tenant := config.GetConfig().GetString("openstack.tenant_name")
	domain := config.GetConfig().GetString("openstack.domain_name")

	return NewKeystoneBackend(authURL, tenant, domain)
}
