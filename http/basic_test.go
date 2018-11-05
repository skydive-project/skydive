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
	"net/http"
	"testing"

	auth "github.com/abbot/go-http-auth"
)

type fakeResponseWriter struct {
	headers http.Header
	body    []byte
	status  int
}

func (r *fakeResponseWriter) Header() http.Header {
	return r.headers
}

func (r *fakeResponseWriter) Write(body []byte) (int, error) {
	r.body = body
	return len(body), nil
}

func (r *fakeResponseWriter) WriteHeader(status int) {
	r.status = status
}

func TestBasicAuthenticate(t *testing.T) {
	provider := NewHtpasswdMapProvider(map[string]string{"user1": "pass1"})

	basic, err := NewBasicAuthenticationBackend("basic", provider.SecretProvider(), DefaultUserRole)
	if err != nil {
		t.Error(err)
	}

	var called bool
	handler := basic.Wrap(func(w http.ResponseWriter, r *auth.AuthenticatedRequest) { called = true })

	w := &fakeResponseWriter{headers: make(http.Header)}
	r := &http.Request{Header: make(http.Header)}
	r.SetBasicAuth("user1", "pass1")

	checkAuth := func(creds bool) *http.Cookie {
		handler(w, r)

		if !called {
			t.Fatal("The wrapped function should have been called")
		}

		r.Header = http.Header{"Cookie": w.Header()["Set-Cookie"]}

		// check that permissions have been set by creds authentication
		if creds {
			if _, err = r.Cookie("permissions"); err != nil {
				t.Fatal("permissions cookie not found in the response")
			}
		}

		cookie, err := r.Cookie(tokenName)
		if err != nil {
			t.Fatal("authentication token cookie not found in the response")
		}

		return cookie
	}

	// first check first username/password
	cookie := checkAuth(true)

	// reset w and r
	w = &fakeResponseWriter{headers: make(http.Header)}
	r = &http.Request{Header: make(http.Header)}

	// add authentication token cookie
	r.AddCookie(cookie)

	// second check with authentication cookie
	checkAuth(false)
}
