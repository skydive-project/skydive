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
	"net"
	//"net/http"
	"strconv"

	//"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/flow"
	"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/mappings"
	"github.com/redhat-cip/skydive/storage/elasticsearch"
	//"github.com/redhat-cip/skydive/topology"
	//topo_analyzer "github.com/redhat-cip/skydive/topology/analyzer"
)

var quit chan bool

func getInterfaceMappingDrivers() ([]mappings.InterfaceMappingDriver, error) {
	drivers := []mappings.InterfaceMappingDriver{}

	/*neutron, err := mappings.NewNeutronMapper()
	if err != nil {
		return drivers, err
	}
	drivers = append(drivers, neutron)*/

	return drivers, nil
}

func handleMessage(conn *net.UDPConn, analyzer *analyzer.Analyzer) {
	data := make([]byte, 4096)

	for {
		n, _, err := conn.ReadFromUDP(data)
		if err != nil {
			logging.GetLogger().Error("Error while reading: %s", err.Error())
			return
		}

		f, err := flow.FromData(data[0:n])
		if err != nil {
			logging.GetLogger().Error("Error while parsing flow: %s", err.Error())
		}

		analyzer.AnalyzeFlows([]*flow.Flow{f})
	}
}

func main() {
	filename := flag.String("conf", "/etc/skydive/skydive.ini",
		"Config file with all the skydive parameter.")
	flag.Parse()

	err := config.InitConfig(*filename)
	if err != nil {
		panic(err)
	}

	quit = make(chan bool)

	/*go func() {
		mn := topology.NewMultiNodeTopology()
		topo_analyzer.RegisterAnalyzers(mn)

		port, err := config.GetConfig().Section("analyzer").Key("listen").Int64()
		if err != nil {
			panic(err)
		}

		router := mux.NewRouter().StrictSlash(true)
		topology.RegisterStaticEndpoints(mn, router)
		topology.RegisterRpcEndpoints(mn, router)
		http.ListenAndServe(":"+strconv.FormatInt(port, 10), router)
	}()*/

	mapper := mappings.NewFlowMapper()
	drivers, err := getInterfaceMappingDrivers()
	if err != nil {
		panic(err)
	}
	mapper.SetInterfaceMappingDrivers(drivers)

	elasticsearch := elasticseach.GetInstance("127.0.0.1", 9200)

	analyzer := analyzer.NewAnalyzer(mapper, elasticsearch)

	port, err := config.GetConfig().Section("analyzer").Key("listen").Int64()
	if err != nil {
		panic(err)
	}

	addr, err := net.ResolveUDPAddr("udp", ":"+strconv.FormatInt(port, 10))
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	go handleMessage(conn, analyzer)

	fmt.Println("Skydive Agent started !")
	<-quit
}
