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
	"net/http"
	"os"

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	"github.com/redhat-cip/skydive/mappings"
	"github.com/redhat-cip/skydive/ovs"
	"github.com/redhat-cip/skydive/sensors"
	"github.com/redhat-cip/skydive/topology"
)

var quit chan bool

func getInterfaceMappingDrivers(topo *topology.Topology) ([]mappings.InterfaceMappingDriver, error) {
	drivers := []mappings.InterfaceMappingDriver{}

	netlink, err := mappings.NewNetLinkMapper(topo)
	if err != nil {
		return drivers, err
	}
	drivers = append(drivers, netlink)

	/* need to be added after the netlink one since it relies on it */
	ovs, err := mappings.NewOvsMapper(topo)
	if err != nil {
		return drivers, err
	}
	drivers = append(drivers, ovs)

	return drivers, nil
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

	sflowSensor := sensors.NewSFlowSensor("127.0.0.1", 6345)

	ovsSFlowSensor := ovsdb.SFlowSensor{
		ID:         "SkydiveSFlowSensor",
		Interface:  "eth0",
		Target:     sflowSensor.GetTarget(),
		HeaderSize: 256,
		Sampling:   1,
		Polling:    0,
	}
	sflowHandler := ovsdb.NewOvsSFlowSensorsHandler([]ovsdb.SFlowSensor{ovsSFlowSensor})

	ovsmon := ovsdb.NewOvsMonitor("127.0.0.1", 6400)
	ovsmon.AddMonitorHandler(sflowHandler)

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	topo := topology.NewTopology(hostname)
	global := topology.NewGlobalTopology()
	global.Add(topo)

	ns := topology.NewNetNSTopoUpdater(topo)
	ns.Start()

	root := topo.NewNetNs("root")
	nl := topology.NewNetLinkTopoUpdater(root)
	nl.Start()

	ovs := topology.NewOvsTopoUpdater(topo, ovsmon)
	ovs.Start()

	mapper := mappings.NewFlowMapper()
	drivers, err := getInterfaceMappingDrivers(topo)
	if err != nil {
		panic(err)
	}
	mapper.SetInterfaceMappingDrivers(drivers)
	sflowSensor.SetFlowMapper(mapper)

	analyzer, err := analyzer.NewClient("127.0.0.1", 8888)
	if err != nil {
		panic(err)
	}
	sflowSensor.SetAnalyzerClient(analyzer)
	sflowSensor.Start()

	ovsmon.StartMonitoring()

	router := mux.NewRouter().StrictSlash(true)
	topology.RegisterStaticEndpoints(global, router)
	topology.RegisterRpcEndpoints(global, router)
	http.ListenAndServe(":8080", router)

	fmt.Println("Skydive Agent started !")
	<-quit
}
