/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package config

import (
	"crypto/tls"

	"github.com/skydive-project/skydive/common"
)

// GetTLSClientConfig returns TLS config to be used by client
func GetTLSClientConfig(setupRootCA bool) (*tls.Config, error) {
	certPEM := GetString("tls.client_cert")
	keyPEM := GetString("tls.client_key")
	var tlsConfig *tls.Config
	if certPEM != "" && keyPEM != "" {
		var err error
		tlsConfig, err = common.SetupTLSClientConfig(certPEM, keyPEM)
		if err != nil {
			return nil, err
		}
		if setupRootCA {
			rootCaPEM := GetString("tls.ca_cert")
			tlsConfig.RootCAs, err = common.SetupTLSLoadCA(rootCaPEM)
			if err != nil {
				return nil, err
			}
		}
	}
	return tlsConfig, nil
}

// GetTLSServerConfig returns TLS config to be used by server
func GetTLSServerConfig(setupRootCA bool) (*tls.Config, error) {
	certPEM := GetString("tls.server_cert")
	keyPEM := GetString("tls.server_key")

	tlsConfig, err := common.SetupTLSServerConfig(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	if setupRootCA {
		rootCaPEM := GetString("tls.ca_cert")
		tlsConfig.ClientCAs, err = common.SetupTLSLoadCA(rootCaPEM)
		if err != nil {
			return nil, err
		}
	}
	return tlsConfig, nil
}
