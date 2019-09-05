/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"github.com/skydive-project/skydive/common"
)

type resolveMulti struct {
	resolvers []Resolver
}

// NewResolveMulti creates a new name resolver
func NewResolveMulti(resolvers ...Resolver) Resolver {
	return &resolveMulti{
		resolvers: resolvers,
	}
}

// IPToName resolve ip address to name
func (rm *resolveMulti) IPToName(ipString, nodeTID string) (string, error) {
	for _, r := range rm.resolvers {
		if name, err := r.IPToName(ipString, nodeTID); err == nil {
			return name, nil
		}
	}

	return "", common.ErrNotFound
}

// TIDToType resolve tid to type
func (rm *resolveMulti) TIDToType(nodeTID string) (string, error) {
	for _, r := range rm.resolvers {
		if ty, err := r.TIDToType(nodeTID); err == nil {
			return ty, nil
		}
	}

	return "", common.ErrNotFound
}
