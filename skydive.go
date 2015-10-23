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

	"github.com/redhat-cip/skydive/agents"
	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/mappings"
	"github.com/redhat-cip/skydive/ovs"
	"github.com/redhat-cip/skydive/storage/elasticsearch"
)

var quit chan bool

func main() {
	filename := flag.String("conf", "/etc/skydive/skydive.ini",
		"Config file with all the skydive parameter.")
	flag.Parse()

	err := config.InitConfig(*filename)
	if err != nil {
		panic(err)
	}

	quit = make(chan bool)

	mapper, err := mappings.NewNeutronMapper()
	if err != nil {
		panic(err)
	}

	storage := elasticseach.GetInstance("127.0.0.1", 9200)
	analyzer := analyzer.New(mapper, storage)

	sflow := agents.NewSFlowAgent("127.0.0.1", 6343, analyzer)
	go sflow.Start()

	agent := ovsdb.SFlowAgent{
		Id:         "SkydiveSFlowAgent",
		Interface:  "eth0",
		Agent:      sflow,
		HeaderSize: 128,
		Sampling:   1,
		Polling:    0,
	}
	agents := []ovsdb.Agent{agent}

	ovsdb.StartBridgesMonitor("127.0.0.1", 6400, agents)

	fmt.Println("Skydive Agent started !")
	<-quit
}
