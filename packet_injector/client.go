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

package packet_injector

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	shttp "github.com/skydive-project/skydive/http"
)

type PacketInjectorReply struct {
	TrackingID string
	Error      string
}

type PacketInjectorClient struct {
	WSServer *shttp.WSMessageServer
}

func (pc *PacketInjectorClient) InjectPacket(host string, pp *PacketParams) (string, error) {
	msg := shttp.NewWSMessage(Namespace, "PIRequest", pp)

	resp, err := pc.WSServer.Request(host, msg, shttp.DefaultRequestTimeout)
	if err != nil {
		return "", fmt.Errorf("Unable to send message to agent %s: %s", host, err.Error())
	}

	var reply PacketInjectorReply
	if err := json.Unmarshal([]byte(*resp.Obj), &reply); err != nil {
		return "", fmt.Errorf("Failed to parse response from %s: %s", host, err.Error())
	}

	if resp.Status != http.StatusOK {
		return "", errors.New(reply.Error)
	}

	return reply.TrackingID, nil
}

func NewPacketInjectorClient(w *shttp.WSMessageServer) *PacketInjectorClient {
	return &PacketInjectorClient{WSServer: w}
}
