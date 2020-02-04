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
	"errors"
	"fmt"
	"net/http"
	"strings"

	auth "github.com/abbot/go-http-auth"
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	tokens2 "github.com/gophercloud/gophercloud/openstack/identity/v2/tokens"
	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/mitchellh/mapstructure"
)

// KeystoneAuthenticationBackend describes a Keystone based authentication backend.
// It authenticates user against either V2 or V3 Keystone server.
type KeystoneAuthenticationBackend struct {
	AuthURL string
	Tenant  string
	Domain  string
	name    string
	role    string
}

// User describes the 'user' structure returned by the Keystone API
type User struct {
	ID   string `mapstructure:"id"`
	Name string `mapstructure:"name"`
}

// Name returns the name of the backend
func (b *KeystoneAuthenticationBackend) Name() string {
	return b.name
}

// DefaultUserRole return the default user role
func (b *KeystoneAuthenticationBackend) DefaultUserRole(user string) string {
	return b.role
}

// SetDefaultUserRole defines the default user role
func (b *KeystoneAuthenticationBackend) SetDefaultUserRole(role string) {
	b.role = role
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
		return "", fmt.Errorf("Tenant mismatch")
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
	project := response.Token.Project
	if project.Name != b.Tenant {
		return "", fmt.Errorf("Tenant mismatch")
	} else if project.Domain.Name != b.Domain {
		return "", fmt.Errorf("Domain mismatch")
	}

	return response.Token.User.Name, nil
}

// CheckUser returns the user authenticated by a token
func (b *KeystoneAuthenticationBackend) CheckUser(token string) (string, error) {
	provider, err := openstack.NewClient(b.AuthURL)
	if err != nil {
		return "", err
	}
	provider.TokenID = token

	client := &gophercloud.ServiceClient{
		ProviderClient: provider,
		Endpoint:       b.AuthURL,
	}

	if b.Domain != "" {
		return b.checkUserV3(client, token)
	}

	return b.checkUserV2(client, token)
}

// Authenticate the user and its password
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
		opts.Password = "xxxxxxxxx"
		return "", err
	}

	return provider.TokenID, nil
}

// Wrap an HTTP handler with Keystone authentication
func (b *KeystoneAuthenticationBackend) Wrap(wrapped auth.AuthenticatedHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := tokenFromRequest(r)

		if username, err := b.CheckUser(token); username == "" {
			Unauthorized(w, r, err)
		} else {
			authCallWrapped(w, r, username, wrapped)
		}
	}
}

// NewKeystoneBackend returns a new Keystone authentication backend
func NewKeystoneBackend(name string, authURL string, tenant string, domain string, role string) (*KeystoneAuthenticationBackend, error) {
	if authURL == "" {
		return nil, errors.New("Authentication URL empty")
	}

	if !strings.HasSuffix(authURL, "/") {
		authURL += "/"
	}

	return &KeystoneAuthenticationBackend{
		AuthURL: authURL,
		Tenant:  tenant,
		Domain:  domain,
		name:    name,
		role:    role,
	}, nil
}
