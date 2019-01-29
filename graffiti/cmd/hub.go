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

package cmd

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	api "github.com/skydive-project/skydive/api/server"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/hub"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/cobra"
)

const (
	defaultQueueSize = 10000
)

var (
	hubListen        string
	writeCompression bool
	queueSize        int
	pingDelay        int
	pongTimeout      int
)

// HubCmd describes the graffiti hub command
var HubCmd = &cobra.Command{
	Use:          "hub",
	Short:        "Graffiti hub",
	Long:         "Graffiti hub",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		logging.GetLogger().Noticef("Graffiti hub starting...")

		sa, err := common.ServiceAddressFromString(hubListen)
		if err != nil {
			logging.GetLogger().Errorf("Configuration error: %s", err)
			os.Exit(1)
		}

		hostname, err := os.Hostname()
		if err != nil {
			logging.GetLogger().Errorf("Failed to get hostname: %s", err)
			os.Exit(1)
		}

		persistent, err := graph.NewMemoryBackend()
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		cached, err := graph.NewCachedBackend(persistent)
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		service := common.Service{ID: hostname, Type: "Hub"}
		g := graph.NewGraph(hostname, cached, serviceType)

		httpServer := shttp.NewServer(hostname, service.Type, sa.Addr, sa.Port, nil)

		if err := httpServer.Listen(); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		authBackend := shttp.NewNoAuthenticationBackend()

		// declare all extension available throught API and filtering
		tr := traversal.NewGremlinTraversalParser()
		tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())

		if _, err = api.NewAPI(httpServer, nil, service, authBackend); err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}
		api.RegisterTopologyAPI(httpServer, g, tr, authBackend)

		hub, err := hub.NewHub(httpServer, g, cached, authBackend, authBackend, nil, "/ws/pod", writeCompression, queueSize, time.Second*time.Duration(pingDelay), time.Second*time.Duration(pongTimeout))
		if err != nil {
			logging.GetLogger().Error(err)
			os.Exit(1)
		}

		go httpServer.Serve()

		hub.Start()

		logging.GetLogger().Notice("Graffiti hub started !")
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		logging.GetLogger().Notice("Graffiti hub stopped.")
	},
}

func init() {
	HubCmd.Flags().StringVarP(&hubListen, "listen", "l", "127.0.0.1:8082", "address and port for the hub server")
	HubCmd.Flags().IntVar(&queueSize, "queueSize", 10000, "websocket queue size")
	HubCmd.Flags().IntVar(&pingDelay, "pingDelay", 2, "websocket ping delay")
	HubCmd.Flags().IntVar(&pongTimeout, "pongTimeout", 10, "websocket pong timeout")
}
