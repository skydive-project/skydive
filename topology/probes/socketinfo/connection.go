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

package socketinfo

import (
	"encoding/binary"
	"hash/fnv"
	"net"
	"strconv"

	"github.com/pmylund/go-cache"

	"github.com/skydive-project/skydive/common"
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
type ConnectionInfo struct {
	ProcessInfo   `mapstructure:",squash"`
	LocalAddress  string
	LocalPort     int64
	RemoteAddress string
	RemotePort    int64
	State         ConnectionState
}

// Hash computes the hash of a connection
func (c *ConnectionInfo) Hash() string {
	return HashTuple(net.ParseIP(c.LocalAddress), c.LocalPort, net.ParseIP(c.RemoteAddress), c.RemotePort)
}

// GetFieldInt64 returns the value of a connection field of type int64
func (c *ConnectionInfo) GetFieldInt64(name string) (int64, error) {
	switch name {
	case "Pid":
		return c.Pid, nil
	case "LocalPort":
		return c.LocalPort, nil
	case "RemotePort":
		return c.RemotePort, nil
	default:
		return 0, common.ErrNotFound
	}
}

// GetFieldInt64 returns the value of a connection field of type string
func (c *ConnectionInfo) GetFieldString(name string) (string, error) {
	switch name {
	case "Process":
		return c.Process, nil
	case "Name":
		return c.Name, nil
	case "LocalAddress":
		return c.LocalAddress, nil
	case "RemoteAddress":
		return c.RemoteAddress, nil
	case "State":
		return string(c.State), nil
	default:
		return "", common.ErrNotFound
	}
}

// GetField returns the value of a field
func (c *ConnectionInfo) GetField(field string) (interface{}, error) {
	if i, err := c.GetFieldInt64(field); err == nil {
		return i, nil
	}

	return c.GetFieldString(field)
}

// ConnectionCache describes a cache of TCP connections
type ConnectionCache struct {
	*cache.Cache
}

// HashTuple computes a hash value for the srcAddr:srcPort dstAddr:dstPort tuple
func HashTuple(srcAddr net.IP, srcPort int64, dstAddr net.IP, dstPort int64) string {
	portBytes := make([]byte, 2)
	hasher := fnv.New64()
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
func (c *ConnectionCache) Get(srcAddr, dstAddr *net.TCPAddr) (interface{}, string) {
	hash := HashTuple(srcAddr.IP, int64(srcAddr.Port), dstAddr.IP, int64(dstAddr.Port))
	if obj, found := c.Cache.Get(hash); found {
		return obj, hash
	}
	return nil, hash
}

// Remove the entry for a pair of TCP addresses
func (c *ConnectionCache) Remove(srcAddr, dstAddr *net.TCPAddr) {
	hash := HashTuple(srcAddr.IP, int64(srcAddr.Port), dstAddr.IP, int64(dstAddr.Port))
	c.Cache.Delete(hash)
}

// MapTCP returns the sending and receiving processes for a pair of TCP addresses
func (c *ConnectionCache) MapTCP(srcAddr, dstAddr *net.TCPAddr) (a *ProcessInfo, b *ProcessInfo) {
	if conn, _ := c.Get(srcAddr, dstAddr); conn != nil {
		a = &conn.(*ConnectionInfo).ProcessInfo
	}
	if conn, _ := c.Get(dstAddr, srcAddr); conn != nil {
		b = &conn.(*ConnectionInfo).ProcessInfo
	}
	return
}

// NewConnectionCache returns a new connection cache
func NewConnectionCache() *ConnectionCache {
	return &ConnectionCache{
		Cache: cache.New(cache.NoExpiration, cache.NoExpiration),
	}
}
