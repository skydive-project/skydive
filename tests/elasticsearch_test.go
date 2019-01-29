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

package tests

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	es "github.com/skydive-project/skydive/storage/elasticsearch"
)

type fakeMasterElection struct {
}

func (f *fakeMasterElection) NewElection(name string) common.MasterElection {
	return nil
}

func delTestIndex(name string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("http://localhost:9200/skydive_%s*", name), nil)
	if err != nil {
		return err
	}
	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	return nil
}

func getClient(t *testing.T, indices []es.Index, cfg es.Config) (*es.Client, error) {
	es.SetRollingRate(5 * time.Second)

	client, err := es.NewClient(indices, cfg, &fakeMasterElection{})
	if err != nil {
		return nil, err
	}
	client.Start()

	err = common.Retry(func() error {
		if client.Started() {
			return nil
		}
		return errors.New("Elasticsearch connection not ready")
	}, 10, time.Second)

	if err != nil {
		return nil, err
	}

	return client, nil
}

func TestRollingSimple(t *testing.T) {
	if topologyBackend != "elasticsearch" && flowBackend != "elasticsearch" {
		t.Skip("Elasticsearch is used neither for topology nor flows")
	}

	// del first in order to cleanup previews run
	delTestIndex("rolling")
	delTestIndex("not_rolling")

	indices := []es.Index{
		{
			Name:      "rolling",
			Type:      "object",
			RollIndex: true,
		},
		{
			Name: "not_rolling",
			Type: "object",
		},
	}

	cfg := es.Config{
		ElasticHost:  "localhost:9200",
		EntriesLimit: 10,
	}

	client, err := getClient(t, indices, cfg)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i != 9; i++ {
		id := fmt.Sprintf("before-entry_%d", i)
		client.Index(indices[0], id, map[string]string{"ID": id})
		client.Index(indices[1], id, map[string]string{"ID": id})
	}

	err = common.Retry(func() error {
		for _, index := range indices {
			result, err := client.Search("object", nil, filters.SearchQuery{}, index.Alias())
			if err != nil {
				return err
			}
			if len(result.Hits.Hits) != 9 {
				return fmt.Errorf("Expected entries not found: %+v", result.Hits.Hits)
			}
		}
		return nil
	}, 5, time.Second)

	if err != nil {
		t.Fatal(err)
	}

	// add more entries to trigger the rolling index
	for i := 0; i != 2; i++ {
		id := fmt.Sprintf("after-entry-%d", i)
		client.Index(indices[0], id, map[string]string{"ID": id})
		client.Index(indices[1], id, map[string]string{"ID": id})
	}

	time.Sleep(es.RollingRate)

	err = common.Retry(func() error {
		// test that the index has been rotated
		result, err := client.Search("object", nil, filters.SearchQuery{}, indices[0].Alias())
		if err != nil {
			return err
		}
		if len(result.Hits.Hits) != 0 {
			return fmt.Errorf("Expected 1 entry, found: %+v", result.Hits.Hits)
		}

		// test that using the wilcard we can get all the records
		result, err = client.Search("object", nil, filters.SearchQuery{}, indices[0].IndexWildcard())
		if err != nil {
			return err
		}
		if len(result.Hits.Hits) != 11 {
			return fmt.Errorf("Expected 1 entry, found: %+v", result.Hits.Hits)
		}

		// test that the no rolling index has not been rotated
		result, err = client.Search("object", nil, filters.SearchQuery{}, indices[1].Alias())
		if err != nil {
			return err
		}
		if len(result.Hits.Hits) != 11 {
			return fmt.Errorf("Expected 11 entries, found: %+v", result.Hits.Hits)
		}
		return nil
	}, 10, time.Second)

	if err != nil {
		t.Fatal(err)
	}
}
