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

package core

import (
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/websocket"
)

// CfgAuthOpts creates the auth options form configuration
func CfgAuthOpts(cfg *viper.Viper) *shttp.AuthenticationOpts {
	username := cfg.GetString("analyzer.auth.cluster.username")
	password := cfg.GetString("analyzer.auth.cluster.password")
	return &shttp.AuthenticationOpts{
		Username: username,
		Password: password,
	}
}

// NewSubscriber returns a new flow subscriber writing to object store
func NewSubscriber(pipeline *Pipeline, cfg *viper.Viper) (*websocket.StructSpeaker, error) {
	subscriberURLString := cfg.GetString(CfgRoot + "subscriber.url")
	subscriberURL, err := url.Parse(subscriberURLString)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse subscriber URL: %s", err)
	}

	namespace := "flow"
	if captureID := cfg.GetString(CfgRoot + "subscriber.capture_id"); captureID != "" {
		namespace = namespace + "/" + captureID
	}

	wsClient, err := config.NewWSClient(common.AnalyzerService, subscriberURL, websocket.ClientOpts{AuthOpts: CfgAuthOpts(cfg)})
	if err != nil {
		return nil, fmt.Errorf("Failed to create websocket client: %s", err)
	}
	structSpeaker := wsClient.UpgradeToStructSpeaker()
	structSpeaker.AddStructMessageHandler(pipeline, []string{namespace})

	return structSpeaker, nil
}

// SubscriberRun runs the subscriber under main
func SubscriberRun(s *websocket.StructSpeaker) {
	s.Start()
	defer s.Stop()

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
}
