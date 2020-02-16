/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package seed

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/skydive-project/skydive/agent"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/seed"
	"github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	tp "github.com/skydive-project/skydive/topology/probes"
	"github.com/skydive-project/skydive/version"
	"github.com/skydive-project/skydive/websocket"

	"github.com/spf13/cobra"
)

var (
	authenticationOpts http.AuthenticationOpts
	agentAddr          string
	subscriberFilter   string
	rootNode           string
	probeBundle        *probe.Bundle
	probeName          string
)

type seedHandler struct {
	g      *graph.Graph
	probes []string
}

func (s *seedHandler) OnSynchronized() {
	var n *graph.Node
	if rootNode != "" {
		n = s.g.GetNode(graph.Identifier(rootNode))
	}

	if n == nil {
		logging.GetLogger().Errorf("failed to find root node: %s", rootNode)
		os.Exit(1)
	}

	ctx := tp.Context{
		Logger:   logging.GetLogger(),
		Config:   config.GetConfig(),
		Graph:    s.g,
		RootNode: n,
	}

	for _, probeName := range s.probes {
		handler, err := agent.NewTopologyProbe(probeName, ctx, probeBundle)
		if err != nil {
			logging.GetLogger().Errorf("Failed to initialize %s probe: %s", probeName, err)
			os.Exit(1)
		}

		probeBundle.AddHandler(probeName, handler)
	}
	go probeBundle.Start()

	logging.GetLogger().Debugf("%s probe started", probeName)
}

// SeedCmd describe the skydive seed root command
var SeedCmd = &cobra.Command{
	Use:          "seed",
	Short:        "Skydive seed",
	Long:         "Skydive seed [probe1] [probe2] ...",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		config.Set("logging.id", "seed")
		logging.GetLogger().Noticef("Skydive Seed %s starting...", version.Version)

		memory, err := graph.NewMemoryBackend()
		if err != nil {
			logging.GetLogger().Errorf("Failed to start seed: %s", err)
			os.Exit(1)
		}

		hostID := config.GetString("host_id")
		if hostID == "" {
			logging.GetLogger().Errorf("Failed to determine host id for seed")
			os.Exit(1)
		}

		g := graph.NewGraph(hostID, memory, common.UnknownService)

		probeBundle = probe.NewBundle()

		tlsConfig, err := config.GetTLSClientConfig(true)
		if err != nil {
			logging.GetLogger().Errorf("Failed to start seed: %s", err)
			os.Exit(1)
		}

		wsOpts := &websocket.ClientOpts{
			QueueSize:        config.GetInt("http.ws.queue_size"),
			WriteCompression: config.GetBool("http.ws.enable_write_compression"),
			TLSConfig:        tlsConfig,
		}

		seed, err := seed.NewSeed(g, common.SeedService, agentAddr, subscriberFilter, *wsOpts)
		if err != nil {
			logging.GetLogger().Errorf("Failed to start seed: %s", err)
			os.Exit(1)
		}

		seed.AddEventHandler(&seedHandler{g: g, probes: args})
		seed.Start()

		logging.GetLogger().Notice("Skydive seed started")
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		probeBundle.Stop()
		seed.Stop()

		logging.GetLogger().Notice("Skydive seed stopped.")
	},
}

func init() {
	host, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	SeedCmd.Flags().String("host-id", host, "ID used to reference the agent, defaults to hostname")
	config.BindPFlag("host_id", SeedCmd.Flags().Lookup("host-id"))

	SeedCmd.Flags().StringVarP(&authenticationOpts.Username, "username", "", os.Getenv("SKYDIVE_USERNAME"), "username auth parameter")
	SeedCmd.Flags().StringVarP(&authenticationOpts.Password, "password", "", os.Getenv("SKYDIVE_PASSWORD"), "password auth parameter")

	SeedCmd.Flags().StringP("net", "", "", "network namespace")
	SeedCmd.Flags().StringP("mount", "", "", "mount namespace")
	SeedCmd.Flags().StringP("pid", "", "", "PID namespace")
	SeedCmd.Flags().StringP("user", "", "", "user namespace")
	SeedCmd.Flags().StringP("ipc", "", "", "IPC namespace")
	SeedCmd.Flags().StringVarP(&subscriberFilter, "filter", "", "", "gremlin filter")
	SeedCmd.Flags().StringVarP(&rootNode, "root", "", "", "gremlin filter to root node")

	SeedCmd.Flags().StringVarP(&agentAddr, "agent", "", "127.0.0.1:8081", "agent address")
}
