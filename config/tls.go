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

package config

import (
	"crypto/tls"

	"github.com/skydive-project/skydive/common"
)

// GetTLSClientConfig returns TLS config to be used by a server
func GetTLSClientConfig(setupRootCA bool) (*tls.Config, error) {
	certPEM := GetString("agent.X509_cert")
	keyPEM := GetString("agent.X509_key")
	var tlsConfig *tls.Config
	if certPEM != "" && keyPEM != "" {
		var err error
		tlsConfig, err = common.SetupTLSClientConfig(certPEM, keyPEM)
		if err != nil {
			return nil, err
		}
		if setupRootCA {
			analyzerCertPEM := GetString("analyzer.X509_cert")
			tlsConfig.RootCAs, err = common.SetupTLSLoadCertificate(analyzerCertPEM)
			if err != nil {
				return nil, err
			}
		}
	}
	return tlsConfig, nil
}

// GetTLSServerConfig returns TLS config to be used by a server
func GetTLSServerConfig(setupRootCA bool) (*tls.Config, error) {
	certPEM := GetString("analyzer.X509_cert")
	keyPEM := GetString("analyzer.X509_key")
	agentCertPEM := GetString("agent.X509_cert")
	tlsConfig, err := common.SetupTLSServerConfig(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	tlsConfig.ClientCAs, err = common.SetupTLSLoadCertificate(agentCertPEM)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}
