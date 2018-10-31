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
	"github.com/skydive-project/goloxi/of14"
)

// Protocol describes the immutable part of an OpenFlow protocol version
type Protocol interface {
	String() string
	DecodeMessage(data []byte) (goloxi.Message, error)
	NewHello() goloxi.Message
	NewEchoRequest() goloxi.Message
	NewEchoReply() goloxi.Message
	NewBarrierRequest() goloxi.Message
}

// OpenFlow protocols singletons
var (
	OpenFlow10 openFlow10
	OpenFlow14 openFlow14
)

type openFlow10 struct {
}

func (p openFlow10) String() string {
	return "OpenFlow 1.0"
}

func (p openFlow10) NewHello() goloxi.Message {
	return of10.NewHello()
}

func (p openFlow10) NewEchoRequest() goloxi.Message {
	return of10.NewEchoRequest()
}

func (p openFlow10) NewEchoReply() goloxi.Message {
	return of10.NewEchoReply()
}

func (p openFlow10) NewBarrierRequest() goloxi.Message {
	return of10.NewBarrierRequest()
}

func (p openFlow10) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of10.DecodeMessage(data)
}

type openFlow14 struct {
}

func (p openFlow14) String() string {
	return "OpenFlow 1.4"
}

func (p openFlow14) NewHello() goloxi.Message {
	msg := of14.NewHello()
	elem := of14.NewHelloElemVersionbitmap()
	elem.Length = 8
	bitmap := of14.NewUint32()
	bitmap.Value = (1 << goloxi.VERSION_1_0) | (1 << goloxi.VERSION_1_1) | (1 << goloxi.VERSION_1_2) | (1 << goloxi.VERSION_1_3) | (1 << goloxi.VERSION_1_4)
	elem.Bitmaps = append(elem.Bitmaps, bitmap)
	msg.Elements = append(msg.Elements, elem)
	return msg
}

func (p openFlow14) NewEchoRequest() goloxi.Message {
	return of14.NewEchoRequest()
}

func (p openFlow14) NewEchoReply() goloxi.Message {
	return of14.NewEchoReply()
}

func (p openFlow14) NewBarrierRequest() goloxi.Message {
	return of14.NewBarrierRequest()
}

func (p openFlow14) DecodeMessage(data []byte) (goloxi.Message, error) {
	return of14.DecodeMessage(data)
}
