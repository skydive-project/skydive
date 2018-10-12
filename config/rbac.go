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
	"bufio"
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/casbin/casbin/model"
	"github.com/casbin/casbin/persist"
	etcd "github.com/coreos/etcd/client"
	"github.com/skydive-project/skydive/rbac"
	"github.com/skydive-project/skydive/statics"
)

func loadSection(model model.Model, key string, sec string) {
	getKey := func(i int) string {
		if i == 0 {
			return sec
		}
		return sec + strconv.Itoa(i)
	}

	entries := GetStringSlice("rbac.model." + key)
	for i, entry := range entries {
		model.AddDef(sec, getKey(i), entry)
	}
}

func loadConfigPolicy(model model.Model) {
	policies := GetStringSlice("rbac.policy")
	for _, line := range policies {
		persist.LoadPolicyLine(line, model)
	}
}

func loadPolicy(content []byte, model model.Model) error {
	buf := bufio.NewReader(bytes.NewReader([]byte(content)))
	for {
		line, err := buf.ReadString('\n')
		line = strings.TrimSpace(line)
		persist.LoadPolicyLine(line, model)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func loadStaticPolicy(model model.Model) error {
	content, err := statics.Asset("rbac/policy.csv")
	if err != nil {
		return err
	}

	return loadPolicy(content, model)
}

// InitRBAC inits the RBAC mechanism. It load
// - the model from the configuration
// - a policy on etcd
// - a policy bundled in the executable
// - additional policy rules from the configuration
func InitRBAC(kapi etcd.KeysAPI) error {
	m := model.Model{}
	loadSection(m, "request_definition", "r")
	loadSection(m, "policy_definition", "p")
	loadSection(m, "policy_effect", "e")
	loadSection(m, "matchers", "m")
	loadSection(m, "role_definition", "g")

	return rbac.Init(m, kapi, func(m model.Model) error {
		if err := loadStaticPolicy(m); err != nil {
			return err
		}
		loadConfigPolicy(m)
		return nil
	})
}
