/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package enhancers

import (
	"sync"

	"github.com/pmylund/go-cache"
)

const (
	maxCacheUpdateTry = 10
)

type tidCacheEntry struct {
	sync.RWMutex
	tid string
	try int
}

type tidCache struct {
	*cache.Cache
}

func (c *tidCache) get(key string) (*tidCacheEntry, bool) {
	if entry, f := c.Get(key); f {
		ce := entry.(*tidCacheEntry)
		ce.RLock()
		defer ce.RUnlock()

		if ce.tid != "" || ce.try > maxCacheUpdateTry {
			return ce, f
		}
		return ce, false
	}

	return &tidCacheEntry{}, false
}

func (c *tidCache) set(ce *tidCacheEntry, key, tid string) {
	ce.Lock()
	ce.tid = tid
	ce.try++
	ce.Unlock()

	c.Set(key, ce, cache.DefaultExpiration)
}

func (c *tidCache) del(key string) {
	c.Delete(key)
}
