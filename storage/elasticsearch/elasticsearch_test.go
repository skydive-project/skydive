package elasticsearch

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

const testMapping = `
{
	"dynamic_templates": [
		{
			"strings": {
				"match": "*",
				"match_mapping_type": "string",
				"mapping": {
					"type":       "string",
					"index":      "not_analyzed",
					"doc_values": false
				}
			}
		}
	]
}`

func (c *ElasticSearchClient) indexEntry(id int) (bool, error) {
	return c.Index("test_type", fmt.Sprintf("id%d", id), "{\"key\": \"val\"}")
}

func (c *ElasticSearchClient) cleanupIndices(name string) error {
	if _, err := c.connection.DeleteIndex(fmt.Sprintf("%s_%s*", indexPrefix, name)); err != nil {
		return fmt.Errorf(fmt.Sprintf("Failed to clear test indices: %s", err.Error()))
	}
	return nil
}

func getClient(name string, limits ElasticLimits, mappings []map[string][]byte) (*ElasticSearchClient, error) {
	client, err := NewElasticSearchClientFromConfig()
	if err != nil {
		return nil, err
	}
	if err := client.cleanupIndices(name); err != nil {
		return nil, err
	}
	client.Start(name, mappings, limits)

	return client, nil
}

// test rolling elasticsearch indices based on count limit
func TestElasticsearchShouldRollByCount(t *testing.T) {
	limits := ElasticLimits{}
	limits.entriesLimit = 5

	name := "should_roll_by_count_test"

	client, err := getClient(name, limits, []map[string][]byte{})
	if err != nil {
		t.Fatalf("Initialisation error: %s", err.Error())
	}

	for i := 1; i < limits.entriesLimit; i++ {
		if _, err := client.indexEntry(i); err != nil {
			t.Fatalf("Failed to index entry %d: %s", i, err.Error())
		}
		time.Sleep(1 * time.Second)
		if client.shouldRollIndex() {
			t.Fatalf("Index should not have rolled after %d entries (limit is %d)", i, limits.entriesLimit)
		}
	}

	if _, err = client.indexEntry(limits.entriesLimit); err != nil {
		t.Fatalf("Failed to index entry %d: %s", limits.entriesLimit, err.Error())
	}
	time.Sleep(1 * time.Second)
	if client.shouldRollIndex() {
		t.Fatalf("Index should have rolled after %d entries", limits.entriesLimit)
	}

	if err := client.cleanupIndices(name); err != nil {
		t.Fatalf(err.Error())
	}
}

// test rolling elasticsearch indices based on age limit
func TestElasticsearchShouldRollByAge(t *testing.T) {
	limits := ElasticLimits{}
	limits.ageLimit = 5
	name := "should_roll_by_age_test"

	client, err := getClient(name, limits, []map[string][]byte{})
	if err != nil {
		t.Fatalf("Initialisation error: %s", err.Error())
	}

	time.Sleep(time.Duration(limits.ageLimit-2) * time.Second)
	if client.shouldRollIndex() {
		t.Fatalf("Index should not have rolled after %d seconds (limit is %d)", limits.ageLimit-2, limits.ageLimit)
	}

	time.Sleep(4 * time.Second)
	if !client.shouldRollIndex() {
		t.Fatalf("Index should not have rolled after %d seconds (limit is %d)", limits.ageLimit+2, limits.ageLimit)
	}

	if err := client.cleanupIndices(name); err != nil {
		t.Fatalf(err.Error())
	}
}

// test deletion of rolling elasticsearch indices
func TestElasticsearchDelIndices(t *testing.T) {
	limits := ElasticLimits{}
	limits.indicesLimit = 5
	name := "del_indices_test"

	client, err := getClient(name, limits, []map[string][]byte{})
	if err != nil {
		t.Fatalf("Initialisation error: %s", err.Error())
	}
	firstIndex := client.index.path
	time.Sleep(1 * time.Second)

	for i := 1; i < limits.indicesLimit; i++ {
		if err := client.RollIndex(); err != nil {
			t.Fatalf("Failed to roll index %d: %s", i, err.Error())
		}
		time.Sleep(1 * time.Second)
		indices := client.connection.GetCatIndexInfo(client.GetIndexAlias() + "_*")
		if len(indices) != i+1 {
			t.Fatalf("Should have had %d indices after %d rolls (limit is %d), but have %d", i+1, i, limits.indicesLimit, len(indices))
		}
	}

	if err = client.RollIndex(); err != nil {
		t.Fatalf("Failed to roll index %d: %s", limits.indicesLimit, err.Error())
	}
	time.Sleep(1 * time.Second)
	indices := client.connection.GetCatIndexInfo(client.GetIndexAlias() + "_*")
	if len(indices) != limits.indicesLimit {
		t.Fatalf("Should have had %d indices after %d rolls (limit is %d), but have %d", limits.indicesLimit, limits.indicesLimit, limits.indicesLimit, len(indices))
	}

	for _, esIndex := range indices {
		if esIndex.Name == firstIndex {
			t.Fatalf("First index %s Should have been deleted", firstIndex)
		}
	}

	if err := client.cleanupIndices(name); err != nil {
		t.Fatalf(err.Error())
	}

}

// test mappings before and after rolling elasticsearch indices
func TestElasticsearchMappings(t *testing.T) {
	limits := ElasticLimits{}
	name := "mappings_test"
	mapKey := "testmap"

	client, err := getClient(name, limits, []map[string][]byte{{mapKey: []byte(testMapping)}})
	if err != nil {
		t.Fatalf("Initialisation error: %s", err.Error())
	}

	if err := client.RollIndex(); err != nil {
		t.Fatalf("Failed to roll index: %s", err.Error())
	}
	time.Sleep(1 * time.Second)

	code, data, err := client.request("GET", fmt.Sprintf("/%s/_mapping", client.GetIndexAlias()), "", "")
	if code != http.StatusOK {
		t.Fatalf("Failed to retreive mappings: %d %s", code, err.Error())
	}

	var mappings map[string]interface{}
	if err := json.Unmarshal(data, &mappings); err != nil {
		t.Fatalf("Unable to parse mappings: %s", err.Error())
	}

	for indexName, doc := range mappings {
		if strings.HasPrefix(indexName, client.GetIndexAlias()) {
			mapping := doc.(map[string]interface{})["mappings"]
			if _, ok := mapping.(map[string]interface{})[mapKey]; !ok {
				t.Fatalf("test mapping not found: %v", mapping)
			}
		}
	}

	if err := client.cleanupIndices(name); err != nil {
		t.Fatalf(err.Error())
	}
}
