/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package alert

import (
	shttp "github.com/redhat-cip/skydive/http"
)

const (
	Namespace = "Alert"
)

type AlertServer struct {
	shttp.DefaultWSServerEventHandler
	WSServer     *shttp.WSServer
	AlertManager *AlertManager
	clients      map[*shttp.WSClient]*alertClient
}

type alertClient struct {
	wsClient *shttp.WSClient
}

/* Called by alert.EvalNodes() */
func (c *alertClient) OnAlert(amsg *AlertMessage) {
	msg := shttp.WSMessage{
		Namespace: Namespace,
		Type:      "Alert",
		Obj:       amsg,
	}

	c.wsClient.SendWSMessage(msg)
}

func (a *AlertServer) OnRegisterClient(c *shttp.WSClient) {
	ac := &alertClient{
		wsClient: c,
	}

	a.clients[c] = ac
	a.AlertManager.AddEventListener(ac)
}

func (a *AlertServer) OnUnregisterClient(c *shttp.WSClient) {
	ac := a.clients[c]
	if ac == nil {
		return
	}

	a.AlertManager.DelEventListener(ac)
	delete(a.clients, c)
}

func NewServer(a *AlertManager, server *shttp.WSServer) *AlertServer {
	s := &AlertServer{
		AlertManager: a,
		WSServer:     server,
		clients:      make(map[*shttp.WSClient]*alertClient),
	}
	server.AddEventHandler(s)

	return s
}
