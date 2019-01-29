/*
 * Copyright (C) 2017 Red Hat, Inc.
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
	"net"
	"testing"

	"github.com/skydive-project/skydive/flow"
)

func TestConnectionCache(t *testing.T) {
	addr1 := &net.TCPAddr{
		IP:   net.IPv4(127, 0, 0, 1),
		Port: 1234,
	}
	addr2 := &net.TCPAddr{
		IP:   net.IPv4(8, 8, 8, 8),
		Port: 80,
	}

	c := NewConnectionCache()
	conn := &ConnectionInfo{
		LocalAddress:  addr1.IP.String(),
		LocalPort:     int64(addr1.Port),
		RemoteAddress: addr2.IP.String(),
		RemotePort:    int64(addr2.Port),
		Protocol:      flow.FlowProtocol_TCP,
	}
	c.Set(conn.Hash(), conn)

	if c, _ := c.Get(flow.FlowProtocol_TCP, addr1.IP, addr1.Port, addr2.IP, addr2.Port); c == nil {
		t.Errorf("Expected entry for %s -> %s", addr1.String(), addr2.String())
	}

	c.Remove(flow.FlowProtocol_TCP, addr1, addr2)

	if c, _ := c.Get(flow.FlowProtocol_TCP, addr1.IP, addr1.Port, addr2.IP, addr2.Port); c != nil {
		t.Errorf("No entry expected for %s -> %s, got %+v", addr1.String(), addr2.String(), c)
	}
}
