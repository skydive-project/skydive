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

package elasticseach

import (
	"strconv"

	elastigo "github.com/mattbaird/elastigo/lib"

	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
)

type ElasticSearchStorage struct {
	connection *elastigo.Conn
}

func (c *ElasticSearchStorage) StoreFlows(flows []*flow.Flow) error {
	/* TODO(safchain) bulk insert */
	for _, flow := range flows {
		logging.GetLogger().Debug("Indexing: %s", flow)
		_, err := c.connection.Index("skydive", "flow", flow.Uuid, nil, *flow)
		if err != nil {
			logging.GetLogger().Error("Error while indexing: %s", err)
			continue
		}
	}

	return nil
}

func GetInstance(addr string, port int) *ElasticSearchStorage {
	c := elastigo.NewConn()
	c.Domain = addr
	c.Port = strconv.FormatInt(int64(port), 10)

	storage := &ElasticSearchStorage{connection: c}
	return storage
}
