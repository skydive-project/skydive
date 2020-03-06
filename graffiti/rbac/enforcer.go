/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package rbac

import (
	"github.com/casbin/casbin"
	"github.com/casbin/casbin/model"
	etcd "github.com/coreos/etcd/client"
)

// Permission defines a permission
type Permission struct {
	Object  string
	Action  string
	Allowed bool
}

var enforcer *casbin.SyncedEnforcer

// Init loads the model from the configuration file then the policies.
// 3 policies are applied, in that order :
// - the policy uploaded in etcd and shared by all analyzers
// - a policy bundled into the binary
// - a policy specified in the configuration file
func Init(model model.Model, kapi etcd.KeysAPI, loadPolicy func(model.Model) error) error {
	etcdAdapter, err := NewEtcdAdapter(kapi)
	if err != nil {
		return err
	}

	casbinEnforcer := casbin.NewSyncedEnforcer()
	casbinEnforcer.InitWithModelAndAdapter(model, etcdAdapter)

	if loadPolicy != nil {
		if err := loadPolicy(model); err != nil {
			return err
		}
	}
	casbinEnforcer.BuildRoleLinks()

	watcher := NewEtcdWatcher(kapi)

	watcher.SetUpdateCallback(func(string) {
		casbinEnforcer.LoadPolicy()
		if loadPolicy != nil {
			loadPolicy(model)
		}
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

// AddRoleForUser registers a role for a user
func AddRoleForUser(user, role string) bool {
	if enforcer == nil {
		return false
	}

	return enforcer.AddRoleForUser(user, role)
}

// GetUserRoles returns the roles of a user
func GetUserRoles(user string) []string {
	if enforcer == nil {
		return []string{}
	}

	return enforcer.GetRolesForUser(user)
}

// GetPermissionsForUser returns all the allow and deny permissions for a user
func GetPermissionsForUser(user string) []Permission {
	if enforcer == nil {
		return nil
	}

	subjects := enforcer.GetRolesForUser(user)
	subjects = append(subjects, user)

	mperms := make(map[string]Permission)
	for _, subject := range subjects {
		for _, p := range enforcer.GetPermissionsForUser(subject) {
			permission := Permission{Object: p[1], Action: p[2], Allowed: p[3] == "allow"}

			key := permission.Object + permission.Action
			mperms[key] = permission
		}
	}

	var permissions []Permission
	for _, permission := range mperms {
		permissions = append(permissions, permission)
	}

	return permissions
}
