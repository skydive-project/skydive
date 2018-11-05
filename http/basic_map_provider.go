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

package http

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"

	"github.com/GehirnInc/crypt"
	// Specifically import md5_crypt
	_ "github.com/GehirnInc/crypt/md5_crypt"
	auth "github.com/abbot/go-http-auth"
)

// HtpasswdMapProvider defines a basic auth secret provider
type HtpasswdMapProvider struct {
	sync.RWMutex
	users map[string]string
}

// AddUser add a new user with the given password
func (h *HtpasswdMapProvider) AddUser(user, password string) {
	h.Lock()
	h.users[user] = password
	h.Unlock()
}

// SecretProvider returns a SecretProvider
func (h *HtpasswdMapProvider) SecretProvider() auth.SecretProvider {
	return func(user, realm string) string {
		h.RLock()
		password, ok := h.users[user]
		h.RUnlock()
		if !ok {
			return ""
		}

		salt := make([]byte, 5)
		io.ReadFull(rand.Reader, salt)

		crypt := crypt.MD5.New()
		hash, err := crypt.Generate([]byte(password), []byte("$1$"+hex.EncodeToString(salt)+"$"))
		if err != nil {
			return ""
		}

		return hash
	}
}

// NewHtpasswdMapProvider creates a new htpassword provider based on a map
func NewHtpasswdMapProvider(users map[string]string) *HtpasswdMapProvider {
	if users == nil {
		users = make(map[string]string)
	}

	return &HtpasswdMapProvider{
		users: users,
	}
}
