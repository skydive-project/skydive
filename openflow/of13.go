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
	"github.com/skydive-project/goloxi/of13"
)

// OpenFlow13 implements the 1.3 OpenFlow protocol basic methods
var OpenFlow13 OpenFlow13Protocol

// OpenFlow13Protocol implements the basic methods for OpenFlow 1.3
type OpenFlow13Protocol struct {
}

// String returns the OpenFlow protocol version as a string
func (p OpenFlow13Protocol) String() string {
	return "OpenFlow 1.3"
}

// GetVersion returns the OpenFlow protocol wire version
func (p OpenFlow13Protocol) GetVersion() uint8 {
	return goloxi.VERSION_1_3
}

// NewHello returns a new hello message
func (p OpenFlow13Protocol) NewHello(versionBitmap uint32) goloxi.Message {
	msg := of13.NewHello()
	elem := of13.NewHelloElemVersionbitmap()
	elem.Length = 8
	bitmap := of13.NewUint32()
	bitmap.Value = versionBitmap
	elem.Bitmaps = append(elem.Bitmaps, bitmap)
	msg.Elements = append(msg.Elements, elem)
	return msg
}

// NewEchoRequest returns a new echo request message
func (p OpenFlow13Protocol) NewEchoRequest() goloxi.Message {
	return of13.NewEchoRequest()
}

// NewEchoReply returns a new echo reply message
func (p OpenFlow13Protocol) NewEchoReply() goloxi.Message {
	return of13.NewEchoReply()
}

// NewBarrierRequest returns a new barrier request message
func (p OpenFlow13Protocol) NewBarrierRequest() goloxi.Message {
	return of13.NewBarrierRequest()
}

// DecodeMessage parses an OpenFlow message
func (p OpenFlow13Protocol) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of13.DecodeMessage(data)
}
