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

package common

import (
	"net"
	"strings"
)

type ServiceType string

const (
	AnalyzerService ServiceType = "analyzer"
	AgentService    ServiceType = "agent"
)

const (
	StoppedState = iota + 1
	RunningState
	StoppingState
)

type ServiceAddress struct {
	Addr string
	Port int
}

func (st ServiceType) String() string {
	return string(st)
}

func ServiceAddressFromString(addressPort string) (ServiceAddress, error) {
	/* Backward compatibility for old format like : listen = 1234 */
	if !strings.ContainsAny(addressPort, ".:") {
		addressPort = ":" + addressPort
	}
	/* validate IPv4 and IPv6 address */
	IPAddr, err := net.ResolveUDPAddr("", addressPort)
	if err != nil {
		return ServiceAddress{}, err
	}
	IPaddr := IPAddr.IP
	port := IPAddr.Port

	addr := "localhost"
	if IPaddr != nil {
		addr = IPToString(IPaddr)
	}
	return ServiceAddress{
		Addr: addr,
		Port: port,
	}, nil
}
