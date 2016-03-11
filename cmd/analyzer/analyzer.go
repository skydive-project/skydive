/*
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package analyzer

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage/elasticsearch"

	"github.com/spf13/cobra"
)

var Analyzer = &cobra.Command{
	Use:          "analyzer",
	Short:        "Skydive analyzer",
	Long:         "Skydive analyzer",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		logging.GetLogger().Notice("Skydive Analyzer starting...")

		router := mux.NewRouter().StrictSlash(true)

		server, err := analyzer.NewServerFromConfig(router)
		if err != nil {
			logging.GetLogger().Fatalf("Can't start Analyzer : %v", err)
		}

		storage, err := elasticseach.New()
		if err != nil {
			logging.GetLogger().Fatalf("Can't connect to ElasticSearch server : %v", err)
		}
		server.SetStorage(storage)

		logging.GetLogger().Notice("Skydive Analyzer started !")
		go server.ListenAndServe()

		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		server.Stop()
		storage.Close()

		logging.GetLogger().Notice("Skydive Analyzer stopped.")
	},
}

func init() {
	Analyzer.Flags().String("listen", "127.0.0.1:8082", "address and port for the analyzer API")
	config.GetConfig().BindPFlag("analyzer.listen", Analyzer.Flags().Lookup("listen"))

	Analyzer.Flags().Int("flowtable-expire", 10, "expiration time for flowtable entries")
	config.GetConfig().BindPFlag("analyzer.flowtable_expire", Analyzer.Flags().Lookup("flowtable-expire"))

	Analyzer.Flags().String("elasticsearch", "127.0.0.1:9200", "elasticsearch server")
	config.GetConfig().BindPFlag("storage.elasticsearch", Analyzer.Flags().Lookup("elasticsearch"))

	Analyzer.Flags().String("etcd", "http://127.0.0.1:2379", "etcd servers")
	config.GetConfig().BindPFlag("etcd.servers", Analyzer.Flags().Lookup("etcd"))

	Analyzer.Flags().Bool("embed-etcd", true, "embed etcd")
	config.GetConfig().BindPFlag("etcd.embedded", Analyzer.Flags().Lookup("embed-etcd"))

	Analyzer.Flags().Int("etcd-port", 2379, "embedded etcd port")
	config.GetConfig().BindPFlag("etcd.port", Analyzer.Flags().Lookup("etcd-port"))

	Analyzer.Flags().String("etcd-datadir", "/tmp/skydive-etcd", "embedded etcd data folder")
	config.GetConfig().BindPFlag("etcd.data_dir", Analyzer.Flags().Lookup("etcd-datadir"))

	Analyzer.Flags().String("graph-backend", "memory", "graph backend")
	config.GetConfig().BindPFlag("graph.backend", Analyzer.Flags().Lookup("graph-backend"))

	Analyzer.Flags().String("gremlin", "ws://127.0.0.1:8182", "gremlin server")
	config.GetConfig().BindPFlag("graph.gremlin", Analyzer.Flags().Lookup("gremlin"))
}
