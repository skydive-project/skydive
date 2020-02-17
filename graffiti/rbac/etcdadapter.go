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
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"

	"github.com/casbin/casbin/model"
	"github.com/casbin/casbin/persist"
	"github.com/casbin/casbin/util"
	etcd "github.com/coreos/etcd/client"
)

const etcdPolicyKey = "/casbinPolicy"

// EtcdAdapter represents the etcd adapter for policy persistence, can load policy
// from etcd or save policy to etcd.
type EtcdAdapter struct {
	kapi etcd.KeysAPI
}

// NewEtcdAdapter is the constructor for EtcdAdapter.
func NewEtcdAdapter(kapi etcd.KeysAPI) (*EtcdAdapter, error) {
	return &EtcdAdapter{kapi: kapi}, nil
}

// LoadPolicy loads policy from etcd.
func (a *EtcdAdapter) LoadPolicy(model model.Model) error {
	resp, err := a.kapi.Get(context.Background(), etcdPolicyKey, nil)
	if err != nil {
		return err
	}

	buf := bufio.NewReader(bytes.NewReader([]byte(resp.Node.Value)))
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

// SavePolicy saves policy to etcd.
func (a *EtcdAdapter) SavePolicy(model model.Model) error {
	var tmp bytes.Buffer

	for ptype, ast := range model["p"] {
		for _, rule := range ast.Policy {
			tmp.WriteString(ptype + ", ")
			tmp.WriteString(util.ArrayToString(rule))
			tmp.WriteString("\n")
		}
	}

	for ptype, ast := range model["g"] {
		for _, rule := range ast.Policy {
			tmp.WriteString(ptype + ", ")
			tmp.WriteString(util.ArrayToString(rule))
			tmp.WriteString("\n")
		}
	}

	s := strings.TrimRight(tmp.String(), "\n")
	_, err := a.kapi.Set(context.Background(), etcdPolicyKey, s, nil)
	return err
}

// AddPolicy adds a policy rule to the storage.
func (a *EtcdAdapter) AddPolicy(sec string, ptype string, rule []string) error {
	return errors.New("not implemented")
}

// RemovePolicy removes a policy rule from the storage.
func (a *EtcdAdapter) RemovePolicy(sec string, ptype string, rule []string) error {
	return errors.New("not implemented")
}

// RemoveFilteredPolicy removes policy rules that match the filter from the storage.
func (a *EtcdAdapter) RemoveFilteredPolicy(sec string, ptype string, fieldIndex int, fieldValues ...string) error {
	return errors.New("not implemented")
}
