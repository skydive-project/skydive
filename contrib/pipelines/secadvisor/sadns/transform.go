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

package sadns

import (
	"strconv"

	"github.com/skydive-project/skydive/contrib/pipelines/core"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/layers"
)

// DNSPort defines DNS service port
const DNSPort = 53

// SecurityAdvisorFlowLayer is the flow layer for a security advisor flow
type SecurityAdvisorFlowLayer struct {
	Protocol string `json:"Protocol,omitempty"`
	A        string `json:"A,omitempty"`
	B        string `json:"B,omitempty"`
}

// DNSHolder holds DNS related data
type DNSHolder struct {
	Network   *SecurityAdvisorFlowLayer
	Transport *SecurityAdvisorFlowLayer
	DNS       *layers.DNS
}

// DNSTransformer retrieves DNS data from flows and returns array of packets
type DNSTransformer struct {
}

// Transform transforms DNS data before storing
func (dt *DNSTransformer) Transform(f *flow.Flow) interface{} {
	if f == nil {
		return nil
	}
	if len(f.DNS) == 0 {
		return nil
	}
	dfdns := make([]*DNSHolder, len(f.DNS))
	for i, v := range f.DNS {
		var Network *SecurityAdvisorFlowLayer
		var Transport *SecurityAdvisorFlowLayer
		if f.Transport.B == DNSPort {
			Network = &SecurityAdvisorFlowLayer{
				Protocol: f.Network.Protocol.String(),
				A:        f.Network.A,
				B:        f.Network.B}
			Transport = &SecurityAdvisorFlowLayer{
				Protocol: f.Transport.Protocol.String(),
				A:        strconv.FormatInt(f.Transport.A, 10),
				B:        strconv.FormatInt(f.Transport.B, 10)}
		} else {
			Network = &SecurityAdvisorFlowLayer{
				Protocol: f.Network.Protocol.String(),
				A:        f.Network.B,
				B:        f.Network.A}
			Transport = &SecurityAdvisorFlowLayer{
				Protocol: f.Transport.Protocol.String(),
				A:        strconv.FormatInt(f.Transport.B, 10),
				B:        strconv.FormatInt(f.Transport.A, 10)}
		}
		DNS := &layers.DNS{
			ID:           v.ID,
			QR:           v.QR,
			OpCode:       v.OpCode,
			AA:           v.AA,
			TC:           v.TC,
			RD:           v.RD,
			RA:           v.RA,
			Z:            v.Z,
			ResponseCode: v.ResponseCode,
			QDCount:      v.QDCount,
			ANCount:      v.ANCount,
			NSCount:      v.NSCount,
			ARCount:      v.ARCount,
			Questions:    v.Questions,
			Answers:      v.Answers,
			Authorities:  v.Authorities,
			Additionals:  v.Additionals,
			Timestamp:    v.Timestamp}

		dfdns[i] = &DNSHolder{
			Network:   Network,
			Transport: Transport,
			DNS:       DNS}
	}
	// after DNS data has been reported delete it to avoid reporting it in the future
	f.DNS = []*layers.DNS{}
	return dfdns
}

// NewTransformDNS creates a new DNS transformer
func NewTransformDNS() (core.Transformer, error) {
	return &DNSTransformer{}, nil
}
