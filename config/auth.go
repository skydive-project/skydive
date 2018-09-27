/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package config

import (
	"errors"
	"fmt"
	"os"

	auth "github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
)

// NewAuthenticationBackendByName creates a new auth backend based on the name
func NewAuthenticationBackendByName(name string) (backend shttp.AuthenticationBackend, err error) {
	typ := GetString("auth." + name + ".type")
	switch typ {
	case "basic":
		role := GetString("auth." + name + ".role")
		if role == "" {
			role = shttp.DefaultUserRole
		}

		var provider auth.SecretProvider
		if file := GetString("auth." + name + ".file"); file != "" {
			if _, err := os.Stat(file); err != nil {
				return nil, err
			}

			provider = auth.HtpasswdFileProvider(file)
		} else if users := GetStringMapString("auth." + name + ".users"); users != nil && len(users) > 0 {
			provider = shttp.NewHtpasswdMapProvider(users).SecretProvider()
		} else {
			return nil, errors.New("No htpassword provider set, you set either file or inline sections")
		}

		backend, err = shttp.NewBasicAuthenticationBackend(name, provider, role)
	case "keystone":
		authURL := GetString("auth." + name + ".auth_url")
		domain := GetString("auth." + name + ".domain_name")
		tenant := GetString("auth." + name + ".tenant_name")

		role := GetString("auth." + name + ".role")
		if role == "" {
			role = shttp.DefaultUserRole
		}

		backend, err = shttp.NewKeystoneBackend(name, authURL, tenant, domain, role)
	case "noauth":
		backend = shttp.NewNoAuthenticationBackend()
	default:
		err = fmt.Errorf("Authentication type unknown or backend not defined for: %s", name)
	}

	if err != nil {
		return nil, err
	}
	return backend, nil
}
