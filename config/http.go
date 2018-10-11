/*
 * Copyright (C) 2018 Red Hat, Inc.
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
	"fmt"

	"github.com/skydive-project/skydive/common"
	shttp "github.com/skydive-project/skydive/http"
)

// NewHTTPServer returns a new HTTP server based on the configuration
func NewHTTPServer(serviceType common.ServiceType) (*shttp.Server, error) {
	sa, err := common.ServiceAddressFromString(GetString(serviceType.String() + ".listen"))
	if err != nil {
		return nil, fmt.Errorf("Configuration error: %s", err)
	}

	var tlsConfig *tls.Config
	if IsTLSEnabled() {
		tlsConfig, err = GetTLSServerConfig(true)
		if err != nil {
			return nil, err
		}
	}

	return shttp.NewServer(GetString("host_id"), serviceType, sa.Addr, sa.Port, tlsConfig), nil
}
