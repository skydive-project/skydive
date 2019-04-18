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
	"github.com/skydive-project/goloxi/of15"
)

// OpenFlow15 implements the 1.5 OpenFlow protocol basic methods
var OpenFlow15 OpenFlow15Protocol

// OpenFlow15Protocol implements the basic methods for OpenFlow 1.5
type OpenFlow15Protocol struct {
}

// String returns the OpenFlow protocol version as a string
func (p OpenFlow15Protocol) String() string {
	return "OpenFlow 1.5"
}

// GetVersion returns the OpenFlow protocol wire version
func (p OpenFlow15Protocol) GetVersion() uint8 {
	return goloxi.VERSION_1_5
}

// NewHello returns a new hello message
func (p OpenFlow15Protocol) NewHello(versionBitmap uint32) goloxi.Message {
	msg := of15.NewHello()
	elem := of15.NewHelloElemVersionbitmap()
	elem.Length = 8
	bitmap := of15.NewUint32()
	bitmap.Value = versionBitmap
	elem.Bitmaps = append(elem.Bitmaps, bitmap)
	msg.Elements = append(msg.Elements, elem)
	return msg
}

// NewEchoRequest returns a new echo request message
func (p OpenFlow15Protocol) NewEchoRequest() goloxi.Message {
	return of15.NewEchoRequest()
}

// NewEchoReply returns a new echo reply message
func (p OpenFlow15Protocol) NewEchoReply() goloxi.Message {
	return of15.NewEchoReply()
}

// NewBarrierRequest returns a new barrier request message
func (p OpenFlow15Protocol) NewBarrierRequest() goloxi.Message {
	return of15.NewBarrierRequest()
}

// DecodeMessage parses an OpenFlow message
func (p OpenFlow15Protocol) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of15.DecodeMessage(data)
}
