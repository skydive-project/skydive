/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package http

import (
	"crypto/tls"
	"net/http"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

func setTLSHeader(w http.ResponseWriter, r *http.Request) {
	if r.TLS != nil {
		w.Header().Add("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
	}
}

func getTLSConfig(setupRootCA bool) *tls.Config {
	certPEM := config.GetString("agent.X509_cert")
	keyPEM := config.GetString("agent.X509_key")
	var tlsConfig *tls.Config = nil
	if certPEM != "" && keyPEM != "" {
		tlsConfig = common.SetupTLSClientConfig(certPEM, keyPEM)
		if setupRootCA {
			analyzerCertPEM := config.GetString("analyzer.X509_cert")
			tlsConfig.RootCAs = common.SetupTLSLoadCertificate(analyzerCertPEM)
		}
		checkTLSConfig(tlsConfig)
	}
	return tlsConfig
}

func checkTLSConfig(tlsConfig *tls.Config) {
	tlsConfig.InsecureSkipVerify = config.GetBool("agent.X509_insecure")
	if tlsConfig.InsecureSkipVerify == true {
		logging.GetLogger().Warning("======> You running the agent in Insecure, the certificate can't be verified, generally use for test purpose, Please make sure it what's you want <======\n PRODUCTION must not run in Insecure\n")
	}
}
