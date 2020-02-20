//go:generate sh -c "go run github.com/gomatic/renderizer --name=alert --resource=alert --type=Alert --title=Alert --article=an swagger_operations.tmpl > alert_swagger.go"
//go:generate sh -c "go run github.com/gomatic/renderizer --name=alert --resource=alert --type=Alert --title=Alert swagger_definitions.tmpl > alert_swagger.json"

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package server

import (
	"time"

	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/api/rest"
	api "github.com/skydive-project/skydive/graffiti/api/server"
	shttp "github.com/skydive-project/skydive/http"
)

// AlertResourceHandler aims to creates and manage a new Alert.
type AlertResourceHandler struct {
	rest.ResourceHandler
}

// AlertAPIHandler aims to exposes the Alert API.
type AlertAPIHandler struct {
	rest.BasicAPIHandler
}

// New creates a new alert
func (a *AlertResourceHandler) New() rest.Resource {
	return &types.Alert{
		CreateTime: time.Now().UTC(),
	}
}

// Name returns resource name "alert"
func (a *AlertResourceHandler) Name() string {
	return "alert"
}

// RegisterAlertAPI registers an Alert's API to a designated API Server
func RegisterAlertAPI(apiServer *api.Server, authBackend shttp.AuthenticationBackend) (*AlertAPIHandler, error) {
	alertAPIHandler := &AlertAPIHandler{
		BasicAPIHandler: rest.BasicAPIHandler{
			ResourceHandler: &AlertResourceHandler{},
			EtcdKeyAPI:      apiServer.EtcdKeyAPI,
		},
	}
	if err := apiServer.RegisterAPIHandler(alertAPIHandler, authBackend); err != nil {
		return nil, err
	}
	return alertAPIHandler, nil
}
