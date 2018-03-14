// +build !linux

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

package common

const (
	AllPackets = iota
	OnlyIPPackets
)

// RawSocket describes a raw socket C implemenation
type RawSocket struct {
}

// GetFd returns the file descriptor
func (s *RawSocket) GetFd() int {
	return 0
}

// Write outputs some bytes to the file
func (s *RawSocket) Write(data []byte) (int, error) {
	return 0, ErrNotImplemented
}

// Close the file descriptor
func (s *RawSocket) Close() error {
	return ErrNotImplemented
}

// NewRawSocket creates a raw socket for the network interface ifName
func NewRawSocket(ifName string, protocol int) (*RawSocket, error) {
	return nil, ErrNotImplemented
}

// NewRawSocketInNs create/open a socket in the namespace nsPath for the network interface ifName
func NewRawSocketInNs(nsPath string, ifName string, protocol int) (*RawSocket, error) {
	return nil, ErrNotImplemented
}
