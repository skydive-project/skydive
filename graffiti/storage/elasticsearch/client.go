/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package elasticsearch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	version "github.com/hashicorp/go-version"
	elastic "github.com/olivere/elastic/v7"
	esconfig "github.com/olivere/elastic/v7/config"

	etcd "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/filters"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/storage"
)

const (
	schemaVersion  = "12"
	indexPrefix    = "skydive"
	minimalVersion = "5.5"
)

// Config describes configuration for elasticsearch
type Config struct {
	ElasticHost  string
	BulkMaxDelay int
	EntriesLimit int
	AgeLimit     int
	IndicesLimit int
	NoSniffing   bool
}

// ClientInterface describes the mechanism API of ElasticSearch database client
type ClientInterface interface {
	Index(index Index, id string, data interface{}) error
	BulkIndex(index Index, id string, data interface{}) error
	Get(index Index, id string) (*elastic.GetResult, error)
	Delete(index Index, id string) (*elastic.DeleteResponse, error)
	BulkDelete(index Index, id string) error
	Search(query elastic.Query, pagination filters.SearchQuery, indices ...string) (*elastic.SearchResult, error)
	Start()
	AddEventListener(listener storage.EventListener)
	UpdateByScript(query elastic.Query, script *elastic.Script, indices ...string) error
}

// Index defines a Client Index
type Index struct {
	Name      string
	Mapping   string
	RollIndex bool
	URL       string
}

// Client describes a ElasticSearch client connection
type Client struct {
	sync.RWMutex
	url           *url.URL
	esClient      *elastic.Client
	bulkProcessor *elastic.BulkProcessor
	started       atomic.Value
	cfg           Config
	indices       map[string]Index
	rollService   *rollIndexService
	listeners     []storage.EventListener
}

var (
	// ErrBadConfig error bad configuration file
	ErrBadConfig = func(reason string) error { return fmt.Errorf("Config file is misconfigured: %s", reason) }
	// ErrIndexTypeNotFound error index type used but not defined
	ErrIndexTypeNotFound = errors.New("Index type not found in the indices map")
)

// FullName returns the full name of an index, prefix, name, version, suffix in case of rolling index
func (i *Index) FullName() string {
	var suffix string
	if i.RollIndex {
		suffix = "-000001"
	}
	return indexPrefix + "_" + i.Name + "_v" + schemaVersion + suffix
}

// Alias returns the Alias of the index
func (i *Index) Alias() string {
	return indexPrefix + "_" + i.Name
}

// IndexWildcard returns the Index wildcard search string used to all the indexes of an index
// definition. Useful to request rolled over indexes.
func (i *Index) IndexWildcard() string {
	return indexPrefix + "_" + i.Name + "_v" + schemaVersion + "*"
}

func (c *Client) createAliases(index Index) error {
	aliasServer := c.esClient.Alias()
	aliasServer.Add(index.FullName(), index.Alias())
	if _, err := aliasServer.Do(context.Background()); err != nil {
		return err
	}

	return nil
}

func (c *Client) addMapping(index Index) error {
	if _, err := c.esClient.PutMapping().Index(index.FullName()).BodyString(index.Mapping).Do(context.Background()); err != nil {
		return fmt.Errorf("Unable to create %s mapping: %s", index.Mapping, err)
	}
	return nil
}

func (c *Client) createIndices() error {
	for _, index := range c.indices {
		if exists, _ := c.esClient.IndexExists(index.FullName()).Do(context.Background()); !exists {
			if _, err := c.esClient.CreateIndex(index.FullName()).Do(context.Background()); err != nil {
				return fmt.Errorf("Unable to create the skydive index: %s", err)
			}

			if index.Mapping != "" {
				if err := c.addMapping(index); err != nil {
					if _, err := c.esClient.DeleteIndex(index.FullName()).Do(context.Background()); err != nil {
						logging.GetLogger().Errorf("Error while deleting indices: %s", err)
					}

					return err
				}
			}

			if err := c.createAliases(index); err != nil {
				if _, err := c.esClient.DeleteIndex(index.FullName()).Do(context.Background()); err != nil {
					logging.GetLogger().Errorf("Error while deleting indices: %s", err)
				}

				return err
			}
		}
	}

	return nil
}

func (c *Client) start() error {
	esConfig, err := esconfig.Parse(c.url.String())
	if err != nil {
		return err
	}

	if c.cfg.NoSniffing {
		esConfig.Sniff = new(bool)
		*esConfig.Sniff = false
	}

	esClient, err := elastic.NewClientFromConfig(esConfig)
	if err != nil {
		return err
	}
	c.esClient = esClient

	bulkProcessor, err := esClient.BulkProcessor().
		After(func(executionId int64, requests []elastic.BulkableRequest, response *elastic.BulkResponse, err error) {
			if err != nil {
				logging.GetLogger().Errorf("Failed to execute bulk query: %s", err)
				return
			}

			if response.Errors {
				logging.GetLogger().Errorf("Failed to insert %d entries", len(response.Failed()))
				for i, fail := range response.Failed() {
					logging.GetLogger().Errorf("Failed to insert entry %d: %v", i, fail.Error)
				}
			}
		}).
		FlushInterval(time.Duration(c.cfg.BulkMaxDelay) * time.Second).
		Do(context.Background())
	if err != nil {
		return err
	}

	c.bulkProcessor = bulkProcessor

	vt, err := esClient.ElasticsearchVersion(c.url.String())
	if err != nil {
		return fmt.Errorf("Unable to get the version: %s", vt)
	}

	v, err := version.NewVersion(vt)
	if err != nil {
		return fmt.Errorf("Unable to parse the version: %s", vt)
	}

	min, _ := version.NewVersion(minimalVersion)
	if v.LessThan(min) {
		return fmt.Errorf("Skydive support only version > %s, found: %s", minimalVersion, vt)
	}

	if err := c.createIndices(); err != nil {
		return fmt.Errorf("Failed to create index: %s", err)
	}

	c.bulkProcessor.Start(context.Background())

	if c.rollService != nil {
		c.rollService.start()
	}

	c.started.Store(true)

	aliases := []string{}
	for _, index := range c.indices {
		aliases = append(aliases, index.Alias())
	}

	logging.GetLogger().Infof("client started for %s", strings.Join(aliases, ", "))

	c.RLock()
	for _, l := range c.listeners {
		l.OnStarted()
	}
	c.RUnlock()

	return nil
}

// FormatFilter creates a ElasticSearch request based on filters
func FormatFilter(filter *filters.Filter, mapKey string) elastic.Query {
	// TODO: remove all this and replace with olivere/elastic queries
	if filter == nil {
		return nil
	}

	prefix := mapKey
	if prefix != "" {
		prefix += "."
	}

	if f := filter.BoolFilter; f != nil {
		queries := make([]elastic.Query, len(f.Filters))
		for i, item := range f.Filters {
			queries[i] = FormatFilter(item, mapKey)
		}
		boolQuery := elastic.NewBoolQuery()
		switch f.Op {
		case filters.BoolFilterOp_NOT:
			return boolQuery.MustNot(queries...)
		case filters.BoolFilterOp_OR:
			return boolQuery.Should(queries...)
		case filters.BoolFilterOp_AND:
			return boolQuery.Must(queries...)
		default:
			return nil
		}
	}

	if f := filter.TermStringFilter; f != nil {
		return elastic.NewTermQuery(prefix+f.Key, f.Value)
	}
	if f := filter.TermInt64Filter; f != nil {
		return elastic.NewTermQuery(prefix+f.Key, f.Value)
	}
	if f := filter.TermBoolFilter; f != nil {
		return elastic.NewTermQuery(prefix+f.Key, f.Value)
	}

	if f := filter.RegexFilter; f != nil {
		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(f.Value, "^")
		value = strings.TrimSuffix(value, "$")

		return elastic.NewRegexpQuery(prefix+f.Key, value)
	}

	if f := filter.IPV4RangeFilter; f != nil {
		// NOTE(safchain) as for now the IP fields are not typed as IP
		// use a regex

		// ignore the error at this point it should have been catched earlier
		regex, _ := filters.IPV4CIDRToRegex(f.Value)

		// remove anchors as ES matches the whole string and doesn't support them
		value := strings.TrimPrefix(regex, "^")
		value = strings.TrimSuffix(value, "$")

		return elastic.NewRegexpQuery(prefix+f.Key, value)
	}

	if f := filter.GtInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Gt(f.Value)
	}
	if f := filter.LtInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Lt(f.Value)
	}
	if f := filter.GteInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Gte(f.Value)
	}
	if f := filter.LteInt64Filter; f != nil {
		return elastic.NewRangeQuery(prefix + f.Key).Lte(f.Value)
	}
	if f := filter.NullFilter; f != nil {
		return elastic.NewBoolQuery().MustNot(elastic.NewExistsQuery(prefix + f.Key))
	}
	return nil
}

// Index returns the skydive index
func (c *Client) Index(index Index, id string, data interface{}) error {
	if _, err := c.esClient.Index().Index(index.Alias()).Id(id).BodyJson(data).Do(context.Background()); err != nil {
		return err
	}
	return nil
}

// BulkIndex returns the bulk index from the indexer
func (c *Client) BulkIndex(index Index, id string, data interface{}) error {
	req := elastic.NewBulkIndexRequest().Index(index.Alias()).Id(id).Doc(data)
	c.bulkProcessor.Add(req)

	return nil
}

// Get an object
func (c *Client) Get(index Index, id string) (*elastic.GetResult, error) {
	return c.esClient.Get().Index(index.Alias()).Id(id).Do(context.Background())
}

// Delete an object
func (c *Client) Delete(index Index, id string) (*elastic.DeleteResponse, error) {
	return c.esClient.Delete().Index(index.Alias()).Id(id).Do(context.Background())
}

// BulkDelete an object with the indexer
func (c *Client) BulkDelete(index Index, id string) error {
	req := elastic.NewBulkDeleteRequest().Index(index.Alias()).Id(id)
	c.bulkProcessor.Add(req)

	return nil
}

// UpdateByScript updates the document using the given script
func (c *Client) UpdateByScript(query elastic.Query, script *elastic.Script, indices ...string) error {
	if _, err := c.esClient.UpdateByQuery(indices...).Query(query).Script(script).Do(context.Background()); err != nil {
		return err
	}
	return nil
}

// Search an object
func (c *Client) Search(query elastic.Query, opts filters.SearchQuery, indices ...string) (*elastic.SearchResult, error) {
	searchQuery := c.esClient.
		Search().
		Index(indices...).
		Query(query).
		Size(10000)

	if r := opts.PaginationRange; r != nil {
		if r.To < r.From {
			return nil, errors.New("Incorrect PaginationRange, To < From")
		}
		searchQuery = searchQuery.From(int(r.From)).Size(int(r.To - r.From))
	}

	if opts.Sort {
		searchQuery = searchQuery.SortWithInfo(elastic.SortInfo{
			Field:        opts.SortBy,
			Ascending:    opts.SortOrder != filters.SortOrder_Descending,
			UnmappedType: "date",
		})
	}

	return searchQuery.Do(context.Background())
}

// Start the Elasticsearch client background jobs
func (c *Client) Start() {
	for {
		err := c.start()
		if err == nil {
			break
		}
		logging.GetLogger().Errorf("Elasticsearch not available: %s", err)
		time.Sleep(time.Second)
	}
}

// Stop Elasticsearch background client
func (c *Client) Stop() {
	if c.started.Load() == true {
		if c.rollService != nil {
			c.rollService.stop()
		}

		c.esClient.Stop()
	}
}

// Started is the client already started ?
func (c *Client) Started() bool {
	return c.started.Load() == true
}

func urlFromHost(host string) (*url.URL, error) {
	urlStr := host
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	url, err := url.Parse(urlStr)
	if err != nil || url.Port() == "" {
		return nil, ErrBadConfig(fmt.Sprintf("wrong url format, %s", urlStr))
	}
	return url, nil
}

// GetClient returns the elastic client object
func (c *Client) GetClient() *elastic.Client {
	return c.esClient
}

// AddEventListener add event listener
func (c *Client) AddEventListener(listener storage.EventListener) {
	c.Lock()
	c.listeners = append(c.listeners, listener)
	c.Unlock()
}

// NewClient creates a new ElasticSearch client based on configuration
func NewClient(indices []Index, cfg Config, electionService etcd.MasterElectionService) (*Client, error) {
	url, err := urlFromHost(cfg.ElasticHost)
	if err != nil {
		return nil, err
	}

	indicesMap := make(map[string]Index, 0)
	rollIndices := []Index{}
	for _, index := range indices {
		indicesMap[index.Name] = index

		if index.RollIndex {
			rollIndices = append(rollIndices, index)
		}
	}

	client := &Client{
		url:     url,
		cfg:     cfg,
		indices: indicesMap,
	}

	if len(rollIndices) > 0 {
		client.rollService = newRollIndexService(client, rollIndices, cfg, electionService)
	}

	client.started.Store(false)

	return client, nil
}
