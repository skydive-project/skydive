/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package client

import (
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// NewCrudClientFromConfig creates a new REST client on /api
func NewCrudClientFromConfig(authOptions *shttp.AuthenticationOpts) (*shttp.CrudClient, error) {
	tlsConfig, err := config.GetTLSClientConfig(true)
	if err != nil {
		return nil, err
	}

	sa, err := config.GetOneAnalyzerServiceAddress()
	if err != nil && err != config.ErrNoAnalyzerSpecified {
		logging.GetLogger().Errorf("Unable to parse analyzer client %s", err.Error())
		return nil, err
	}

	return shttp.NewCrudClient(config.GetURL("http", sa.Addr, sa.Port, "/api/"), authOptions, tlsConfig), nil
}

// NewRestClientFromConfig creates a new REST client
func NewRestClientFromConfig(authOptions *shttp.AuthenticationOpts) (*shttp.RestClient, error) {
	tlsConfig, err := config.GetTLSClientConfig(true)
	if err != nil {
		return nil, err
	}

	sa, err := config.GetOneAnalyzerServiceAddress()
	if err != nil && err != config.ErrNoAnalyzerSpecified {
		logging.GetLogger().Errorf("Unable to parse analyzer client %s", err.Error())
		return nil, err
	}

	return shttp.NewRestClient(config.GetURL("http", sa.Addr, sa.Port, "/api/"), authOptions, tlsConfig), nil
}
