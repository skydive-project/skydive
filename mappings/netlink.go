/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package mappings

import (
	"strconv"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/pmylund/go-cache"
	"github.com/vishvananda/netlink"

	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
)

type NetLinkMapper struct {
	cache            *cache.Cache
	cacheUpdaterChan chan uint32
}

func (mapper *NetLinkMapper) cacheUpdater() {
	logging.GetLogger().Debug("Start NetLink cache updater")

	var ifIndex uint32
	for {
		ifIndex = <-mapper.cacheUpdaterChan

		logging.GetLogger().Debug("ifIndex request received: %s", ifIndex)

		var attrs *netlink.LinkAttrs

		link, err := netlink.LinkByIndex(int(ifIndex))
		if err != nil {
			logging.GetLogger().Error("Error while getting interface by index via netling: ", err.Error())
			attrs = &netlink.LinkAttrs{}
		} else {
			attrs = link.Attrs()
		}

		mapper.cache.Set(strconv.Itoa(int(ifIndex)), attrs, cache.DefaultExpiration)
	}
}

func (mapper *NetLinkMapper) Enhance(mac string, attrs *flow.Flow_InterfaceAttributes) {
	a, f := mapper.cache.Get(strconv.Itoa(int(attrs.GetIfIndex())))
	if f {
		la := a.(*netlink.LinkAttrs)

		attrs.IfName = proto.String(la.Name)
		attrs.MTU = proto.Uint32(uint32(la.MTU))

		return
	}

	mapper.cacheUpdaterChan <- attrs.GetIfIndex()
}

func NewNetLinkMapper() (*NetLinkMapper, error) {
	mapper := &NetLinkMapper{}

	expire, err := config.GetConfig().Section("cache").Key("expire").Int()
	if err != nil {
		return nil, err
	}
	cleanup, err := config.GetConfig().Section("cache").Key("cleanup").Int()
	if err != nil {
		return nil, err
	}
	mapper.cache = cache.New(time.Duration(expire)*time.Second, time.Duration(cleanup)*time.Second)
	mapper.cacheUpdaterChan = make(chan uint32)
	go mapper.cacheUpdater()

	return mapper, nil
}
