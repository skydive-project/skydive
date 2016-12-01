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

package storage

import (
	"errors"
	"fmt"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/storage/elasticsearch"
	"github.com/skydive-project/skydive/flow/storage/orientdb"
	"github.com/skydive-project/skydive/logging"
)

var (
	NoStorageConfigured error = errors.New("No storage backend has been configured")
)

type Storage interface {
	Start()
	StoreFlows(flows []*flow.Flow) error
	SearchFlows(fsq flow.FlowSearchQuery) (*flow.FlowSet, error)
	SearchMetrics(fsq flow.FlowSearchQuery, metricFilter *flow.Filter) (map[string][]*flow.FlowMetric, error)
	Stop()
}

func NewStorage(backend string) (s Storage, err error) {
	switch backend {
	case "elasticsearch":
		s, err = elasticsearch.New()
		if err != nil {
			logging.GetLogger().Fatalf("Can't connect to ElasticSearch server: %v", err)
		}
	case "orientdb":
		s, err = orientdb.New()
		if err != nil {
			logging.GetLogger().Fatalf("Can't connect to OrientDB server: %v", err)
		}
	case "":
		logging.GetLogger().Infof("Using no storage")
		return
	default:
		err = fmt.Errorf("Storage type unknown: %s", backend)
		logging.GetLogger().Fatalf(err.Error())
		return
	}

	logging.GetLogger().Infof("Using %s as storage", backend)
	return
}

func NewStorageFromConfig() (s Storage, err error) {
	return NewStorage(config.GetConfig().GetString("analyzer.storage"))
}
