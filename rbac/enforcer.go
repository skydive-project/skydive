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

package rbac

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/casbin/casbin"
	"github.com/casbin/casbin/model"
	"github.com/casbin/casbin/persist"
	etcd "github.com/coreos/etcd/client"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/statics"
)

var enforcer *casbin.Enforcer

func loadSection(model model.Model, key string, sec string) {
	getKey := func(i int) string {
		if i == 0 {
			return sec
		}
		return sec + strconv.Itoa(i)
	}

	entries := config.GetStringSlice("rbac.model." + key)
	for i, entry := range entries {
		model.AddDef(sec, getKey(i), entry)
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

func loadConfigPolicy(model model.Model) {
	policies := config.GetStringSlice("rbac.policy")
	for _, line := range policies {
		persist.LoadPolicyLine(line, model)
	}
}

// Init loads the model from the configuration file then the policies.
// 3 policies are applied, in that order :
// - the policy uploaded in etcd and shared by all analyzers
// - a policy bundled into the binary
// - a policy specified in the configuration file
func Init(kapi etcd.KeysAPI) error {
	model := model.Model{}
	loadSection(model, "request_definition", "r")
	loadSection(model, "policy_definition", "p")
	loadSection(model, "policy_effect", "e")
	loadSection(model, "matchers", "m")
	loadSection(model, "role_definition", "g")

	etcdAdapter, err := NewEtcdAdapter(kapi)
	if err != nil {
		return err
	}

	casbinEnforcer := casbin.NewEnforcer()
	casbinEnforcer.InitWithModelAndAdapter(model, etcdAdapter)

	if err := loadStaticPolicy(model); err != nil {
		return err
	}
	loadConfigPolicy(model)

	watcher := NewEtcdWatcher(kapi)

	watcher.SetUpdateCallback(func(string) {
		casbinEnforcer.LoadPolicy()
		loadStaticPolicy(model)
		loadConfigPolicy(model)
		model.PrintPolicy()
		casbinEnforcer.BuildRoleLinks()
	})

	enforcer = casbinEnforcer
	return nil
}

// Enforce decides whether a "subject" can access an "object" with the operation "action"
func Enforce(sub, obj, act string) bool {
	if enforcer == nil {
		return true
	}

	return enforcer.Enforce(sub, obj, act)
}

// GetPermissionsForUser returns all the allow and deny permissions for a user
func GetPermissionsForUser(user string) [][]string {
	if enforcer == nil {
		return nil
	}

	return enforcer.GetPermissionsForUser(user)
}
