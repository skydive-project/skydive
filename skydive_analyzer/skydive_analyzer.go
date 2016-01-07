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

	"github.com/gorilla/mux"

	"github.com/redhat-cip/skydive/analyzer"
	"github.com/redhat-cip/skydive/config"
	//"github.com/redhat-cip/skydive/logging"
	"github.com/redhat-cip/skydive/storage/elasticsearch"
)

func main() {
	filename := flag.String("conf", "/etc/skydive/skydive.ini",
		"Config file with all the skydive parameter.")
	flag.Parse()

	err := config.InitConfig(*filename)
	if err != nil {
		panic(err)
	}

	/*elasticsearch := elasticseach.GetInstance("127.0.0.1", 9200)*/

	port, err := config.GetConfig().Section("analyzer").Key("listen").Int()
	if err != nil {
		panic(err)
	}

	router := mux.NewRouter().StrictSlash(true)

	server, err := analyzer.NewServer(port, router)
	if err != nil {
		panic(err)
	}

	storage, err := elasticseach.New("127.0.0.1", 9200)
	if err != nil {
		panic(err)
	}
	server.SetStorage(storage)

	fmt.Println("Skydive Analyzer started !")
	server.ListenAndServe()
}
