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
	"github.com/skydive-project/skydive/graffiti/logging"
	es "github.com/skydive-project/skydive/graffiti/storage/elasticsearch"
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

	// To be backwards compatible, check if .host key (old) has a string value.
	// In that case, use that value as .hosts (converting the ip:port to http://ip:port)
	// .host will have preference over .hosts
	cfg.ElasticHosts = config.GetStringSlice(path + ".hosts")
	oldElasticHost := config.GetString(path + ".host")
	if oldElasticHost != "" {
		cfg.ElasticHosts = []string{fmt.Sprintf("http://%s", oldElasticHost)}
	}

	cfg.InsecureSkipVerify = config.GetBool(path + ".ssl_insecure")
	cfg.Username = config.GetString(path + ".auth.username")
	cfg.Password = config.GetString(path + ".auth.password")
	cfg.BulkMaxDelay = config.GetInt(path + ".bulk_maxdelay")
	cfg.TotalFieldsLimit = config.GetInt(path + ".total_fields_limit")
	cfg.EntriesLimit = config.GetInt(path + ".index_entries_limit")
	cfg.AgeLimit = config.GetInt(path + ".index_age_limit")
	cfg.IndicesLimit = config.GetInt(path + ".indices_to_keep")
	cfg.NoSniffing = config.GetBool(path + ".disable_sniffing")
	cfg.IndexPrefix = config.GetString(path + ".index_prefix")
	cfg.NoHealthcheck = config.GetBool(path + ".disable_healthcheck")
	cfg.Debug = config.GetBool(path + ".debug")

	return cfg
}

func newGraphBackendFromConfig(etcdClient *etcd.Client) (graph.PersistentBackend, error) {
	backend := config.GetString("analyzer.topology.backend")
	configPath := "storage." + backend
	driver := config.GetString(configPath + ".driver")

	logging.GetLogger().Infof("Using %s (driver %s) as graph storage backend", backend, driver)

	switch driver {
	case "elasticsearch":
		excludeFromIndex := config.GetStringSlice(configPath + ".exclude_from_mapping")
		useFlattened := config.GetBool(configPath + ".use_flattened")

		var mapping map[string]interface{}
		if useFlattened {
			mapping = map[string]interface{}{
				"type":         "flattened",
				"ignore_above": config.GetInt(configPath + ".flattened_ignore_above"),
			}
		} else {
			mapping = map[string]interface{}{
				"type":    "object",
				"enabled": false,
				"store":   true,
				"index":   false,
			}
		}

		dynamicTemplates := make(map[string]interface{})
		for _, field := range excludeFromIndex {
			dynamicTemplates[field] = map[string]interface{}{
				"path_match": field,
				"mapping":    mapping,
			}
		}

		cfg := NewESConfig(backend)
		return graph.NewElasticSearchBackendFromConfig(cfg, dynamicTemplates, etcdClient, logging.GetLogger())
	case "memory":
		// cached memory will be used
		return nil, nil
	case "orientdb":
		addr := config.GetString(configPath + ".addr")
		database := config.GetString(configPath + ".database")
		username := config.GetString(configPath + ".username")
		password := config.GetString(configPath + ".password")
		return graph.NewOrientDBBackend(addr, database, username, password, logging.GetLogger())
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
