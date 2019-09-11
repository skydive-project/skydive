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
	"time"

	cache "github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// Resolver resolves values for the transformer
type Resolver interface {
	IPToName(ipString, nodeTID string) (string, error)
	TIDToType(nodeTID string) (string, error)
}

type resolveCache struct {
	resolver  Resolver
	nameCache *cache.Cache
	typeCache *cache.Cache
}

// NewResolveCache creates a new name resolver
func NewResolveCache(resolver Resolver) Resolver {
	return &resolveCache{
		resolver:  resolver,
		nameCache: cache.New(5*time.Minute, 10*time.Minute),
		typeCache: cache.New(5*time.Minute, 10*time.Minute),
	}
}

// IPToName resolve ip address to name
func (rc *resolveCache) IPToName(ipString, nodeTID string) (string, error) {
	cacheKey := ipString + "," + nodeTID
	name, ok := rc.nameCache.Get(cacheKey)

	if !ok {
		var err error
		name, err = rc.resolver.IPToName(ipString, nodeTID)
		if err != nil {
			if err != common.ErrNotFound {
				logging.GetLogger().Warningf("Failed to query container name for IP '%s': %s", ipString, err)
			}
			return "", err
		}

		rc.nameCache.Set(cacheKey, name, cache.DefaultExpiration)
	}

	return name.(string), nil
}

// TIDToType resolve tid to type
func (rc *resolveCache) TIDToType(nodeTID string) (string, error) {
	nodeType, ok := rc.typeCache.Get(nodeTID)
	if !ok {
		var err error
		nodeType, err = rc.resolver.TIDToType(nodeTID)

		if err != nil {
			if err != common.ErrNotFound {
				logging.GetLogger().Warningf("Failed to query node type for TID '%s': %s", nodeTID, err)
			}
			return "", err
		}

		rc.typeCache.Set(nodeTID, nodeType, cache.DefaultExpiration)
	}

	return nodeType.(string), nil
}
