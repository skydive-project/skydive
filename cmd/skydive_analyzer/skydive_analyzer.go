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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage/elasticsearch"
)

func usage() {
	fmt.Printf("Usage: %s -conf <config.ini> [-h]\n", os.Args[0])
}

func main() {
	filename := flag.String("conf", "/etc/skydive/skydive.ini",
		"Config file with all the skydive parameter.")
	flag.CommandLine.Usage = usage
	flag.Parse()

	err := config.InitConfigFromFile(*filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

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

	logging.GetLogger().Notice("Skydive Analyzer stopped.")
}
