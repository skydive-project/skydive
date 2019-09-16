/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"strconv"

	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/flow"
)

const (
	version   = 2
	accountID = "12345678"
)

type Action string

const (
	ActionReject Action = "REJECT"
	ActionAccept Action = "ACCEPT"
)

type LogStatus string

const (
	LogStatusOk       LogStatus = "OK"
	LogStatusNoData   LogStatus = "NODATA"
	LogStatusSkipData LogStatus = "SKIPDATA"
)

// record struct is based on
// https://docs.aws.amazon.com/vpc/latest/userguide/flow-logs.html)
type record struct {
	// The VPC Flow Logs version.
	Version int `csv:"version"`
	// The AWS account ID for the flow log.
	AccountID string `csv:"account_id"`
	// The ID of the network interface for which the traffic is recorded.
	InterfaceID string `csv:"interface_id"`
	// The source IPv4 or IPv6 address. The IPv4 address of the network
	// interface is always its private IPv4 address.
	SrcAddr string `csv:"srcadr"`
	// The destination IPv4 or IPv6 address. The IPv4 address of the
	// network interface is always its private IPv4 address.
	DstAddr string `csv:"dstaddr"`
	// The source port of the traffic.
	SrcPort int `csv:"srcport"`
	// The destination port of the traffic.
	DstPort int `csv:"dstport"`
	// The IANA protocol number of the traffic. For more information, see
	// Assigned Internet Protocol Numbers.
	Protocol int `csv:"protocol"`
	// The number of packets transferred during the capture window.
	Packets int `csv:"packets"`
	// The number of bytes transferred during the capture window.
	Bytes int64 `csv:"bytes"`
	// The time, in Unix seconds, of the start of the capture window.
	Start int `csv:"start"`
	// The time, in Unix seconds, of the end of the capture window.
	End int `csv:"end"`
	// The recorded traffic: ACCEPT if permitted: REJECT if not permitted
	// (by security groups of network ACLS).
	Action Action `csv:"action"`
	// The logging status: OK if logged normally; NODATA if no data during
	// cature window; SKIPDATA due to possible cap of traffic.
	LogStatus LogStatus `csv:"log_status"`
}

type transform struct {
}

// NewTransform creates a new transformer
func NewTransform(cfg *viper.Viper) (interface{}, error) {
	return &transform{}, nil
}

func getProtocol(f *flow.Flow) int {
	if f.Transport == nil {
		return 0
	}

	return int(f.Transport.Protocol.Value())
}

func getNetworkA(f *flow.Flow) string {
	if f.Network == nil {
		return ""
	}
	return f.Network.A
}

func getNetworkB(f *flow.Flow) string {
	if f.Network == nil {
		return ""
	}
	return f.Network.B
}

func getTransportA(f *flow.Flow) int {
	if f.Transport == nil {
		return 0
	}
	return int(f.Transport.A)
}

func getTransportB(f *flow.Flow) int {
	if f.Transport == nil {
		return 0
	}
	return int(f.Transport.B)
}

// Transform transforms a flow before being stored
func (t *transform) Transform(f *flow.Flow) interface{} {
	return &record{
		Version:     version,
		AccountID:   accountID,
		InterfaceID: strconv.FormatInt(f.Link.ID, 10),
		SrcAddr:     getNetworkA(f),
		DstAddr:     getNetworkB(f),
		SrcPort:     getTransportA(f),
		DstPort:     getTransportB(f),
		Protocol:    getProtocol(f),
		Packets:     int(f.Metric.ABPackets + f.Metric.BAPackets),
		Bytes:       f.Metric.ABBytes + f.Metric.BABytes,
		Start:       int(f.Start / 1000),
		End:         int(f.Last / 1000),
		Action:      ActionAccept,
		LogStatus:   LogStatusOk,
	}
}
