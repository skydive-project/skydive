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

package openflow

import (
	"github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of10"
)

// OpenFlow10 implements the 1.0 OpenFlow protocol basic methods
var OpenFlow10 OpenFlow10Protocol

// OpenFlow10Protocol implements the basic methods for OpenFlow 1.0
type OpenFlow10Protocol struct {
}

// String returns the OpenFlow protocol version as a string
func (p OpenFlow10Protocol) String() string {
	return "OpenFlow 1.0"
}

// GetVersion returns the OpenFlow protocol wire version
func (p OpenFlow10Protocol) GetVersion() uint8 {
	return goloxi.VERSION_1_0
}

// NewHello returns a new hello message
func (p OpenFlow10Protocol) NewHello(versionBitmap uint32) goloxi.Message {
	return of10.NewHello()
}

// NewEchoRequest returns a new echo request message
func (p OpenFlow10Protocol) NewEchoRequest() goloxi.Message {
	return of10.NewEchoRequest()
}

// NewEchoReply returns a new echo reply message
func (p OpenFlow10Protocol) NewEchoReply() goloxi.Message {
	return of10.NewEchoReply()
}

// NewBarrierRequest returns a new barrier request message
func (p OpenFlow10Protocol) NewBarrierRequest() goloxi.Message {
	return of10.NewBarrierRequest()
}

// DecodeMessage parses an OpenFlow message
func (p OpenFlow10Protocol) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of10.DecodeMessage(data)
}
