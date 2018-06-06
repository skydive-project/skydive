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
	"bytes"
	"testing"

	capturer "github.com/kami-zh/go-capturer"
)

func TestRelocation(t *testing.T) {
	cfg.SetConfigType("yaml")

	relocationMap = map[string][]string{
		"openstack.auth_url": {
			"auth.keystone.auth_url",
			"agent.topology.neutron.auth_url",
		},
		"openstack.tenant_name": {
			"auth.keystone.tenant_name",
			"agent.topology.neutron.tenant_name",
		},
	}

	var yamlv1 = []byte(`
auth:
  keystone:
    auth_url: "http://test.test"
    tenant_name: "test"
`)

	cfg.ReadConfig(bytes.NewBuffer(yamlv1))
	out := capturer.CaptureStderr(func() {
		value := Get("openstack.auth_url")
		if value != "http://test.test" {
			t.Fatal("Relocation failed")
		}
	})
	if len(out) == 0 {
		t.Fatal("A warning should have been raised")
	}

	value := Get("openstack.tenant_name")
	if value != "test" {
		t.Fatal("Relocation with default failed")
	}

	var yamlv2 = []byte(`
openstack:
  auth_url: "http://test.test"
`)

	cfg.ReadConfig(bytes.NewBuffer(yamlv2))
	out = capturer.CaptureStderr(func() {
		value := Get("openstack.auth_url")
		if value != "http://test.test" {
			t.Fatal("Relocation failed")
		}
	})
	if len(out) != 0 {
		t.Fatal("A warning shouldn't have been raised")
	}

	value = Get("openstack.tenant_name")
	if value != "admin" {
		t.Fatal("Relocation with default failed")
	}
}
