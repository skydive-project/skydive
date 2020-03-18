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

package validator

import (
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/google/gopacket/layers"
	valid "gopkg.in/validator.v2"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/probes"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
	"github.com/skydive-project/skydive/graffiti/schema"
	ge "github.com/skydive-project/skydive/gremlin/traversal"
	"github.com/skydive-project/skydive/statics"
	"github.com/skydive-project/skydive/topology"
)

type validator struct {
	validator       *valid.Validator
	schemaValidator *schema.Validator
}

var (
	// Validator is the global validator for all Skydive resources
	Validator *validator

	//IPNotValid validator
	IPNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not an IP address")}
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
	//CaptureTypeNotValid validator
	CaptureTypeNotValid = func(t string) error {
		return valid.TextErr{Err: fmt.Errorf("Not a valid capture type: %s, available types: %v", t, probes.ProbeTypes)}
	}
	//AddressNotValid validator
	AddressNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not a valid address")}
	}
	//MACNotValid validator
	MACNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not a MAC address")}
	}
	//IPOrCIDRNotValid validator
	IPOrCIDRNotValid = func() error {
		return valid.TextErr{Err: errors.New("Not a IP or CIDR address")}
	}
)

func isValidAddress(v interface{}, param string) error {
	addr, ok := v.(string)
	if !ok {
		return AddressNotValid()
	}

	if addr == "" {
		return nil
	}

	if _, err := common.ServiceAddressFromString(addr); err != nil {
		return AddressNotValid()
	}

	return nil
}

func isIP(v interface{}, param string) error {
	switch v := v.(type) {
	case string:
		/* Parse/Check IPv4 and IPv6 address */
		if v == "" {
			return nil
		} else if n := net.ParseIP(v); n == nil {
			return IPNotValid()
		}
		return nil
	case net.IP:
		return nil
	default:
		return IPNotValid()
	}
}

func isMAC(v interface{}, param string) error {
	switch v := v.(type) {
	case string:
		if v == "" {
			return nil
		} else if _, err := net.ParseMAC(v); err != nil {
			return MACNotValid()
		}
	case net.HardwareAddr:
		if len(v) == 0 {
			return nil
		}
		if len(v) != 6 {
			return MACNotValid()
		}
	}
	return nil
}

func isIPOrCIDR(v interface{}, param string) error {
	switch v := v.(type) {
	case string:
		if v == "" {
			return nil
		}
		if strings.Contains(v, "/") {
			if _, _, err := net.ParseCIDR(v); err != nil {
				return IPOrCIDRNotValid()
			}
		} else {
			if n := net.ParseIP(v); n == nil {
				return IPOrCIDRNotValid()
			}
		}
		return nil
	case net.IPNet:
		return nil
	case net.IP:
		return nil
	default:
		return IPOrCIDRNotValid()
	}
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
	tr.AddTraversalExtension(ge.NewRawPacketsTraversalExtension())
	tr.AddTraversalExtension(ge.NewDescendantsTraversalExtension())
	tr.AddTraversalExtension(ge.NewNextHopTraversalExtension())
	tr.AddTraversalExtension(ge.NewGroupTraversalExtension())

	if _, err := tr.Parse(strings.NewReader(query)); err != nil {
		return GremlinNotValid(err)
	}

	return nil
}

func isGremlinOrEmpty(v interface{}, param string) error {
	query, ok := v.(string)
	if ok && strings.TrimSpace(query) == "" {
		return nil
	}
	return isGremlinExpr(v, param)
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

func isValidWorkflow(v interface{}, param string) error {
	// Check that `v` is valid JS code that returns
	// a promise
	return nil
}

func isValidCaptureType(v interface{}, param string) error {
	typ, ok := v.(string)
	if !ok {
		return CaptureTypeNotValid("null")
	}

	if len(typ) == 0 {
		return nil
	}

	for _, t := range probes.ProbeTypes {
		if t == typ {
			return nil
		}
	}

	return CaptureTypeNotValid(typ)
}

// Validate an object based on previously (at init) registered function
func Validate(kind string, value interface{}) error {
	if err := Validator.validator.Validate(value); err != nil {
		return err
	}

	return Validator.schemaValidator.Validate(kind, value)
}

func init() {
	v := valid.NewValidator()
	v.SetValidationFunc("isIP", isIP)
	v.SetValidationFunc("isMAC", isMAC)
	v.SetValidationFunc("isIPOrCIDR", isIPOrCIDR)
	v.SetValidationFunc("isGremlinExpr", isGremlinExpr)
	v.SetValidationFunc("isGremlinOrEmpty", isGremlinOrEmpty)
	v.SetValidationFunc("isBPFFilter", isBPFFilter)
	v.SetValidationFunc("isValidCaptureHeaderSize", isValidCaptureHeaderSize)
	v.SetValidationFunc("isValidRawPacketLimit", isValidRawPacketLimit)
	v.SetValidationFunc("isValidLayerKeyMode", isValidLayerKeyMode)
	v.SetValidationFunc("isValidWorkflow", isValidWorkflow)
	v.SetValidationFunc("isValidCaptureType", isValidCaptureType)
	v.SetValidationFunc("isValidAddress", isValidAddress)
	v.SetTag("valid")

	nodeSchema, err := statics.Asset("statics/schemas/node.schema")
	if err != nil {
		panic(err)
	}

	edgeSchema, err := statics.Asset("statics/schemas/edge.schema")
	if err != nil {
		panic(err)
	}

	schemaValidator := schema.NewValidator()
	schemaValidator.LoadSchema("node", nodeSchema)
	schemaValidator.LoadSchema("edge", edgeSchema)

	Validator = &validator{
		validator:       v,
		schemaValidator: topology.SchemaValidator,
	}
}
