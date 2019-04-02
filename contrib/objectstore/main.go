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

package main

import (
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/websocket"
)

const defaultConfigurationFile = "/etc/skydive/skydive-objectstore.yml"

func main() {
	if err := config.InitConfig("file", []string{defaultConfigurationFile}); err != nil {
		logging.GetLogger().Errorf("Failed to initialize config: %s", err.Error())
		os.Exit(1)
	}

	if err := config.InitLogging(); err != nil {
		logging.GetLogger().Errorf("Failed to initialize logging system: %s", err.Error())
		os.Exit(1)
	}

	cfg := config.GetConfig()

	endpoint := cfg.GetString("endpoint")
	region := cfg.GetString("region")
	bucket := cfg.GetString("bucket")
	accessKey := cfg.GetString("access_key")
	secretKey := cfg.GetString("secret_key")
	apiKey := cfg.GetString("api_key")
	iamEndpoint := cfg.GetString("iam_endpoint")
	objectPrefix := cfg.GetString("object_prefix")
	subscriberURLString := cfg.GetString("subscriber_url")
	subscriberUsername := cfg.GetString("subscriber_username")
	subscriberPassword := cfg.GetString("subscriber_password")
	maxFlowsPerObject := cfg.GetInt("max_flows_per_object")
	maxSecondsPerObject := cfg.GetInt("max_seconds_per_object")
	maxSecondsPerStream := cfg.GetInt("max_seconds_per_stream")
	maxFlowArraySize := cfg.GetInt("max_flow_array_size")
	flowTransformerName := cfg.GetString("flow_transformer")
	flowClassifier, err := subscriber.NewFlowClassifier(cfg.GetStringSlice("cluster_net_masks"))
	if err != nil {
		logging.GetLogger().Errorf("Cannot initialize flow classifier: %s", err.Error())
		os.Exit(1)
	}

	flowTransformer, err := subscriber.NewFlowTransformer(flowTransformerName)
	if err != nil {
		logging.GetLogger().Errorf("Failed to initialize flow transformer: %s", err.Error())
		os.Exit(1)
	}

	objectStoreClient := subscriber.NewClient(endpoint, region, accessKey, secretKey, apiKey, iamEndpoint)

	authOpts := &shttp.AuthenticationOpts{
		Username: subscriberUsername,
		Password: subscriberPassword,
	}

	subscriberURL, err := url.Parse(subscriberURLString)
	if err != nil {
		logging.GetLogger().Errorf("Failed to parse subscriber URL: %s", err.Error())
		os.Exit(1)
	}

	wsClient, err := config.NewWSClient(common.AnalyzerService, subscriberURL, websocket.ClientOpts{AuthOpts: authOpts})
	if err != nil {
		logging.GetLogger().Errorf("Failed to create websocket client: %s", err)
		os.Exit(1)
	}
	structClient := wsClient.UpgradeToStructSpeaker()

	s := subscriber.New(objectStoreClient, bucket, objectPrefix, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream, flowTransformer, flowClassifier)

	// subscribe to the flow updates
	structClient.AddStructMessageHandler(s, []string{"flow"})
	structClient.Start()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	structClient.Stop()
}
