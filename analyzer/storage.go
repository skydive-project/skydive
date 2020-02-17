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

package analyzer

import (
	"fmt"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow/storage"
	"github.com/skydive-project/skydive/flow/storage/elasticsearch"
	"github.com/skydive-project/skydive/flow/storage/orientdb"
	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/graph"
	es "github.com/skydive-project/skydive/graffiti/storage/elasticsearch"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// NewESConfig returns a new elasticsearch configuration for the given backend name
func NewESConfig(name ...string) es.Config {
	cfg := es.Config{}

	path := "storage."
	if len(name) > 0 {
		path += name[0]
	} else {
		path += "elasticsearch"
	}

	cfg.ElasticHost = config.GetString(path + ".host")
	cfg.BulkMaxDelay = config.GetInt(path + ".bulk_maxdelay")

	cfg.EntriesLimit = config.GetInt(path + ".index_entries_limit")
	cfg.AgeLimit = config.GetInt(path + ".index_age_limit")
	cfg.IndicesLimit = config.GetInt(path + ".indices_to_keep")
	cfg.NoSniffing = config.GetBool(path + ".disable_sniffing")
	cfg.IndexPrefix = config.GetString(path + ".index_prefix")

	return cfg
}

func newGraphBackendFromConfig(etcdClient *etcd.Client) (graph.Backend, error) {
	backend := config.GetString("analyzer.topology.backend")
	configPath := "storage." + backend
	driver := config.GetString(configPath + ".driver")

	logging.GetLogger().Infof("Using %s (driver %s) as graph storage backend", backend, driver)

	switch driver {
	case "elasticsearch":
		cfg := NewESConfig(backend)
		dynamicTemplates := map[string]interface{}{
			"extra": map[string]interface{}{
				"path_match": "*.Extra",
				"mapping": map[string]interface{}{
					"type":    "object",
					"enabled": false,
					"store":   true,
					"index":   false,
				},
			},
			"openflow_actions": map[string]interface{}{
				"path_match": "*.Actions",
				"mapping": map[string]interface{}{
					"type":    "object",
					"enabled": false,
					"store":   true,
					"index":   false,
				},
			},
			"openflow_filters": map[string]interface{}{
				"path_match": "*.Filters",
				"mapping": map[string]interface{}{
					"type":    "object",
					"enabled": false,
					"store":   true,
					"index":   false,
				},
			},
		}
		return graph.NewElasticSearchBackendFromConfig(cfg, dynamicTemplates, etcdClient)
	case "memory":
		// cached memory will be used
		return nil, nil
	case "orientdb":
		addr := config.GetString(configPath + ".addr")
		database := config.GetString(configPath + ".database")
		username := config.GetString(configPath + ".username")
		password := config.GetString(configPath + ".password")
		return graph.NewOrientDBBackend(addr, database, username, password, etcdClient)
	default:
		return nil, fmt.Errorf("Topology backend driver '%s' not supported", driver)
	}
}

// newStorageFromConfig creates a new flow storage based on the backend
func newFlowBackendFromConfig(etcdClient *etcd.Client) (s storage.Storage, err error) {
	backend := config.GetString("analyzer.flow.backend")
	configPath := "storage." + backend
	driver := config.GetString(configPath + ".driver")

	logging.GetLogger().Infof("Using %s (driver %s) as flow storage backend", backend, driver)

	switch driver {
	case "elasticsearch":
		cfg := NewESConfig(backend)
		return elasticsearch.New(cfg, etcdClient)
	case "memory":
		return nil, nil
	case "orientdb":
		return orientdb.New(backend)
	default:
		return nil, fmt.Errorf("Flow backend driver '%s' not supported", driver)
	}
}
