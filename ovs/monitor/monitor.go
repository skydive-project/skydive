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

package monitor

import (
	"context"

	"github.com/skydive-project/goloxi/of10"
	"github.com/skydive-project/skydive/openflow"
)

// Monitor describes a OVS monitor similar to the
// ovs-vsctl monitor watch: command
type Monitor struct {
	*openflow.Client
}

// SendFlowMonitorRequest asks OVS to send events on new/updated/deleted flows
// similar to ovs-ofctl monitor watch:
func (m *Monitor) SendFlowMonitorRequest() error {
	/*
		// For OpenFlow 1.4
		request := []byte{
			0x05, 0x12, 0x00, 0x30, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x3f, 0xff, 0xff,
			0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x01, 0x00, 0x3f, 0xff, 0xff,
			0x00, 0x00, 0xff, 0x00, 0x00, 0x00, 0x00, 0x00,
		}

		_, err := m.Conn.Write(request)
		return err
	*/

	request := of10.NewNiciraFlowMonitorRequest()
	request.MonitorFlags = of10.NxfmfInitial | of10.NxfmfAdd | of10.NxfmfDelete | of10.NxfmfModify | of10.NxfmfActions | of10.NxfmfOwn
	request.Subtype = 2 // FIXME: use NXST_FLOW
	request.OutPort = of10.OFPPNone
	request.TableId = 0xff
	return m.SendMessage(request)
}

// Start the OVS monitor
func (m *Monitor) Start(ctx context.Context) error {
	if err := m.Client.Start(ctx); err != nil {
		return err
	}

	return m.SendFlowMonitorRequest()
}

// NewMonitor returns a new OVS monitor using either a UNIX socket or a TCP socket
func NewMonitor(addr string) (*Monitor, error) {
	client, err := openflow.NewClient(addr, []openflow.Protocol{openflow.OpenFlow10})
	if err != nil {
		return nil, err
	}

	return &Monitor{
		Client: client,
	}, nil
}
