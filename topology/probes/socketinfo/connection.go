//go:generate go run ../../../scripts/gendecoder.go -package github.com/skydive-project/skydive/topology/probes/socketinfo

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

package socketinfo

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"net"
	"reflect"
	"strconv"

	"github.com/mitchellh/mapstructure"
	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/flow"
)

// ProcessInfo describes the information of a running process
type ProcessInfo struct {
	Process string
	Pid     int64
	Name    string
}

// ConnectionState describes the state of a connection
type ConnectionState string

// ConnectionInfo describes a connection and its corresponding process
// easyjson:json
// gendecoder
type ConnectionInfo struct {
	ProcessInfo   `mapstructure:",squash"`
	LocalAddress  string
	LocalPort     int64
	RemoteAddress string
	RemotePort    int64
	Protocol      flow.FlowProtocol
	State         ConnectionState
}

// Hash computes the hash of a connection
func (c *ConnectionInfo) Hash() string {
	return HashTuple(c.Protocol, net.ParseIP(c.LocalAddress), c.LocalPort, net.ParseIP(c.RemoteAddress), c.RemotePort)
}

// Decode an JSON object to connection info
func (c *ConnectionInfo) Decode(obj interface{}) error {
	objMap, ok := obj.(map[string]interface{})
	if !ok {
		return fmt.Errorf("Unable to decode connection: %v, %+v", obj, reflect.TypeOf(obj))
	}

	// copy to not modify original node
	m := make(map[string]interface{})
	for k, v := range objMap {
		m[k] = v
	}

	if protocol, ok := m["Protocol"]; ok {
		if protocol, ok := protocol.(string); ok {
			m["Protocol"] = flow.FlowProtocol_value[protocol]
		}
	}

	return mapstructure.WeakDecode(m, c)
}

// ConnectionCache describes a cache of TCP connections
type ConnectionCache struct {
	*cache.Cache
}

// HashTuple computes a hash value for a connection 5 tuple
func HashTuple(protocol flow.FlowProtocol, srcAddr net.IP, srcPort int64, dstAddr net.IP, dstPort int64) string {
	portBytes := make([]byte, 2)
	protocolBytes := make([]byte, 4)
	hasher := fnv.New64()
	binary.LittleEndian.PutUint32(protocolBytes, uint32(protocol))
	hasher.Write(protocolBytes)
	hasher.Write([]byte(srcAddr.To16()))
	binary.LittleEndian.PutUint16(portBytes, uint16(srcPort))
	hasher.Write(portBytes)
	hasher.Write([]byte(dstAddr.To16()))
	binary.LittleEndian.PutUint16(portBytes, uint16(dstPort))
	hasher.Write(portBytes)
	return strconv.Itoa(int(hasher.Sum64()))
}

// Set maps a hash to a connection
func (c *ConnectionCache) Set(hash string, obj interface{}) {
	c.Cache.Set(hash, obj, cache.NoExpiration)
}

// Get returns the connection for a pair of TCP addresses
func (c *ConnectionCache) Get(protocol flow.FlowProtocol, srcIP net.IP, srcPort int, dstIP net.IP, dstPort int) (interface{}, string) {
	hash := HashTuple(protocol, srcIP, int64(srcPort), dstIP, int64(dstPort))
	if obj, found := c.Cache.Get(hash); found {
		return obj, hash
	}
	return nil, hash
}

// Remove the entry for a pair of TCP addresses
func (c *ConnectionCache) Remove(protocol flow.FlowProtocol, srcAddr, dstAddr *net.TCPAddr) {
	hash := HashTuple(protocol, srcAddr.IP, int64(srcAddr.Port), dstAddr.IP, int64(dstAddr.Port))
	c.Cache.Delete(hash)
}

// Map a flow to a process
func (c *ConnectionCache) Map(protocol flow.FlowProtocol, srcIP net.IP, srcPort int, dstIP net.IP, dstPort int) (a *ProcessInfo, b *ProcessInfo) {
	if conn, _ := c.Get(protocol, srcIP, srcPort, dstIP, dstPort); conn != nil {
		a = &conn.(*ConnectionInfo).ProcessInfo
	}
	if conn, _ := c.Get(protocol, dstIP, dstPort, srcIP, srcPort); conn != nil {
		b = &conn.(*ConnectionInfo).ProcessInfo
	}
	return
}

// MapTCP returns the sending and receiving processes for a pair of TCP addresses
func (c *ConnectionCache) MapTCP(srcAddr, dstAddr *net.TCPAddr) (a *ProcessInfo, b *ProcessInfo) {
	return c.Map(flow.FlowProtocol_TCP, srcAddr.IP, srcAddr.Port, dstAddr.IP, dstAddr.Port)
}

// MapUDP returns the sending and receiving processes for a pair of UDP addresses
func (c *ConnectionCache) MapUDP(srcAddr, dstAddr *net.UDPAddr) (a *ProcessInfo, b *ProcessInfo) {
	return c.Map(flow.FlowProtocol_UDP, srcAddr.IP, srcAddr.Port, dstAddr.IP, dstAddr.Port)
}

// NewConnectionCache returns a new connection cache
func NewConnectionCache() *ConnectionCache {
	return &ConnectionCache{
		Cache: cache.New(cache.NoExpiration, cache.NoExpiration),
	}
}
