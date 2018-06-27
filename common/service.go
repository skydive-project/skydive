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
	"fmt"
	"net"
	"strings"
)

// ServiceType describes the service type (analyzer or agent)
type ServiceType string

const (
	// UnknownService for unknown client types
	UnknownService ServiceType = "unknown"
	// AnalyzerService analyzer
	AnalyzerService ServiceType = "analyzer"
	// AgentService agent
	AgentService ServiceType = "agent"
)

const (
	// StoppedState service stopped
	StoppedState = iota + 1
	// RunningState service running
	RunningState
	// StoppingState service stopping
	StoppingState
)

// ServiceAddress describes the service listening address and port
type ServiceAddress struct {
	Addr string
	Port int
}

func (st ServiceType) String() string {
	return string(st)
}

func (sa ServiceAddress) String() string {
	return fmt.Sprintf("%s:%d", sa.Addr, sa.Port)
}

// ServiceAddressFromString returns a service address from a string, could be IPv4 or IPv6
func ServiceAddressFromString(addressPort string) (ServiceAddress, error) {
	/* Backward compatibility for old format like : listen = 1234 */
	if !strings.ContainsAny(addressPort, ".:") {
		addressPort = "localhost:" + addressPort
	} else if strings.HasPrefix(addressPort, ":") {
		addressPort = "localhost" + addressPort
	}

	host, port, err := net.SplitHostPort(addressPort)
	if err != nil {
		return ServiceAddress{}, err
	}

	portNum, err := net.LookupPort("", port)
	if err != nil {
		return ServiceAddress{}, err
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return ServiceAddress{}, err
	}
	if len(ips) == 0 {
		return ServiceAddress{}, fmt.Errorf("no address found for %s", host)
	}

	// just take the first address returned
	addr := NormalizeIPForURL(ips[0])

	return ServiceAddress{
		Addr: addr,
		Port: portNum,
	}, nil
}
