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

package common

import (
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync/atomic"
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
	// SeedService seed
	SeedService ServiceType = "seed"
)

// ServiceState describes the state of a service.
type ServiceState int64

// MarshalJSON marshal the connection state to JSON
func (s *ServiceState) MarshalJSON() ([]byte, error) {
	switch *s {
	case StartingState:
		return []byte("\"starting\""), nil
	case RunningState:
		return []byte("\"running\""), nil
	case StoppingState:
		return []byte("\"stopping\""), nil
	case StoppedState:
		return []byte("\"stopped\""), nil
	}
	return nil, fmt.Errorf("Invalid state: %d", s)
}

// Store atomatically stores the state
func (s *ServiceState) Store(state ServiceState) {
	atomic.StoreInt64((*int64)(s), int64(state))
}

// Load atomatically loads and returns the state
func (s *ServiceState) Load() ServiceState {
	return ServiceState(atomic.LoadInt64((*int64)(s)))
}

// CompareAndSwap executes the compare-and-swap operation for a state
func (s *ServiceState) CompareAndSwap(old, new ServiceState) bool {
	return atomic.CompareAndSwapInt64((*int64)(s), int64(old), int64(new))
}

const (
	// StoppedState service stopped
	StoppedState ServiceState = iota + 1
	// StartingState service starting
	StartingState
	// RunningState service running
	RunningState
	// StoppingState service stopping
	StoppingState
)

// Service describes a service identified by its type and identifier
type Service struct {
	Type ServiceType
	ID   string
}

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

// NormalizeIPForURL returns a string normalized that can be used in URL. Brackets
// will be used for IPV6 addresses.
func NormalizeIPForURL(ip net.IP) string {
	if ip.To4() == nil {
		return "[" + ip.String() + "]"
	}
	return ip.String()
}

// MakeURL creates an URL for the specified protocol, address, port and path,
// whether TLS is required or not
func MakeURL(protocol string, addr string, port int, path string, useTLS bool) *url.URL {
	u, _ := url.Parse(fmt.Sprintf("%s://%s:%d%s", protocol, addr, port, path))

	if (protocol == "http" || protocol == "ws") && useTLS {
		u.Scheme += "s"
	}

	return u
}
