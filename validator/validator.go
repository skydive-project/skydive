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

package validator

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/gopacket/layers"
	valid "gopkg.in/validator.v2"

	"github.com/skydive-project/skydive/flow"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/topology/graph/traversal"
)

// Validator interface used to validate value type in Gremlin expression
type Validator interface {
	Validate() error
}

var (
	skydiveValidator = valid.NewValidator()

	//IPNotValid validator
	IPNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not a IP addr")}
	}
	// GremlinNotValid validator
	GremlinNotValid = func(err error) error {
		return valid.TextErr{Err: fmt.Errorf("Not a valid Gremlin expression: %s", err)}
	}
	// BPFFilterNotValid validator
	BPFFilterNotValid = func(err error) error {
		return valid.TextErr{Err: fmt.Errorf("Not a valid BPF expression: %s", err)}
	}
	// CaptureHeaderSizeNotValid validator
	CaptureHeaderSizeNotValid = func(min, max uint32) error {
		return valid.TextErr{Err: fmt.Errorf("A valid header size is >= %d && <= %d", min, max)}
	}
	// RawPacketLimitNotValid validator
	RawPacketLimitNotValid = func(min, max uint32) error {
		return valid.TextErr{Err: fmt.Errorf("A valid raw packet limit size is > %d && <= %d", min, max)}
	}

	//LayerKeyModeNotValid validator
	LayerKeyModeNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not a valid layer key mode")}
	}
)

func isIP(v interface{}, param string) error {
	ip, ok := v.(string)
	if !ok {
		return IPNotValid()
	}
	/* Parse/Check IPv4 and IPv6 address */
	if n := net.ParseIP(ip); n == nil {
		return IPNotValid()
	}
	return nil
}

func isGremlinExpr(v interface{}, param string) error {
	query, ok := v.(string)
	if !ok {
		return GremlinNotValid(errors.New("not a string"))
	}

	tr := traversal.NewGremlinTraversalParser()
	tr.AddTraversalExtension(ge.NewMetricsTraversalExtension())
	tr.AddTraversalExtension(ge.NewFlowTraversalExtension(nil, nil))
	tr.AddTraversalExtension(ge.NewSocketsTraversalExtension())

	if _, err := tr.Parse(strings.NewReader(query)); err != nil {
		return GremlinNotValid(err)
	}

	return nil
}

func isBPFFilter(v interface{}, param string) error {
	bpfFilter, ok := v.(string)
	if !ok {
		return BPFFilterNotValid(errors.New("not a string"))
	}

	if _, err := flow.BPFFilterToRaw(layers.LinkTypeEthernet, flow.MaxCaptureLength, bpfFilter); err != nil {
		return BPFFilterNotValid(err)
	}

	return nil
}

func isValidCaptureHeaderSize(v interface{}, param string) error {
	headerSize, ok := v.(int)
	hs := uint32(headerSize)
	if !ok || hs < 0 || (hs > 0 && hs < 14) || hs > flow.MaxCaptureLength {
		return CaptureHeaderSizeNotValid(14, flow.MaxCaptureLength)
	}

	return nil
}

func isValidRawPacketLimit(v interface{}, param string) error {
	limit, ok := v.(int)
	l := uint32(limit)
	// The current limitation of 10 packet could be removed once flow will be
	// transferred over tcp.
	if !ok || l < 0 || l > flow.MaxRawPacketLimit {
		return RawPacketLimitNotValid(0, flow.MaxRawPacketLimit)
	}

	return nil
}

func isValidLayerKeyMode(v interface{}, param string) error {
	name, ok := v.(string)
	if !ok {
		return LayerKeyModeNotValid()
	}

	if len(name) == 0 {
		return nil
	}

	if _, err := flow.LayerKeyModeByName(name); err != nil {
		return LayerKeyModeNotValid()
	}
	return nil
}

// Validate an object based on previously (at init) registered function
func Validate(value interface{}) error {
	if err := skydiveValidator.Validate(value); err != nil {
		return err
	}

	if obj, ok := value.(Validator); ok {
		return obj.Validate()
	}

	return nil
}

func init() {
	skydiveValidator.SetValidationFunc("isIP", isIP)
	skydiveValidator.SetValidationFunc("isGremlinExpr", isGremlinExpr)
	skydiveValidator.SetValidationFunc("isBPFFilter", isBPFFilter)
	skydiveValidator.SetValidationFunc("isValidCaptureHeaderSize", isValidCaptureHeaderSize)
	skydiveValidator.SetValidationFunc("isValidRawPacketLimit", isValidRawPacketLimit)
	skydiveValidator.SetValidationFunc("isValidLayerKeyMode", isValidLayerKeyMode)
	skydiveValidator.SetTag("valid")
}
