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

package subscriber

import (
	"fmt"
	"net/url"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// Subscriber represents a flow subscriber writing to object store
type Subscriber struct {
	structClient *websocket.StructSpeaker
	storage      *Storage
}

// NewSubscriberFromConfig returns a new flow subscriber writing to object store
func NewSubscriberFromConfig(cfg *viper.Viper) (*Subscriber, error) {
	cfgPrefix := "objectstore."

	endpoint := cfg.GetString(cfgPrefix + "endpoint")
	region := cfg.GetString(cfgPrefix + "region")
	bucket := cfg.GetString(cfgPrefix + "bucket")
	accessKey := cfg.GetString(cfgPrefix + "access_key")
	secretKey := cfg.GetString(cfgPrefix + "secret_key")
	apiKey := cfg.GetString(cfgPrefix + "api_key")
	iamEndpoint := cfg.GetString(cfgPrefix + "iam_endpoint")
	objectPrefix := cfg.GetString(cfgPrefix + "object_prefix")
	subscriberURLString := cfg.GetString(cfgPrefix + "subscriber_url")
	subscriberUsername := cfg.GetString(cfgPrefix + "subscriber_username")
	subscriberPassword := cfg.GetString(cfgPrefix + "subscriber_password")
	maxFlowsPerObject := cfg.GetInt(cfgPrefix + "max_flows_per_object")
	maxSecondsPerObject := cfg.GetInt(cfgPrefix + "max_seconds_per_object")
	maxSecondsPerStream := cfg.GetInt(cfgPrefix + "max_seconds_per_stream")
	maxFlowArraySize := cfg.GetInt(cfgPrefix + "max_flow_array_size")
	flowTransformerName := cfg.GetString(cfgPrefix + "flow_transformer")
	flowClassifier, err := newFlowClassifier(cfg.GetStringSlice(cfgPrefix + "cluster_net_masks"))
	if err != nil {
		return nil, fmt.Errorf("Cannot initialize flow classifier: %s", err.Error())
	}
	excludedTags := cfg.GetStringSlice(cfgPrefix + "excluded_tags")

	objectStoreClient := newClient(endpoint, region, accessKey, secretKey, apiKey, iamEndpoint)

	authOpts := &shttp.AuthenticationOpts{
		Username: subscriberUsername,
		Password: subscriberPassword,
	}

	gremlinClient := client.NewGremlinQueryHelper(authOpts)
	flowTransformer, err := newFlowTransformer(flowTransformerName, gremlinClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize flow transformer: %s", err.Error())
	}

	subscriberURL, err := url.Parse(subscriberURLString)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse subscriber URL: %s", err.Error())
	}

	wsClient, err := config.NewWSClient(common.AnalyzerService, subscriberURL, websocket.ClientOpts{AuthOpts: authOpts})
	if err != nil {
		return nil, fmt.Errorf("Failed to create websocket client: %s", err)
	}
	structClient := wsClient.UpgradeToStructSpeaker()

	s := newStorage(objectStoreClient, bucket, objectPrefix, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream, flowTransformer, flowClassifier, excludedTags)

	// subscribe to the flow updates
	structClient.AddStructMessageHandler(s, []string{"flow"})

	return &Subscriber{structClient: structClient, storage: s}, nil
}

// Start starts the object store flow subscriber
func (s *Subscriber) Start() {
	s.structClient.Start()
}

// Stop stops the object store flow subscriber
func (s *Subscriber) Stop() {
	s.structClient.Stop()
}

// GetStorage returns the backing storage client
func (s *Subscriber) GetStorage() *Storage {
	return s.storage
}
