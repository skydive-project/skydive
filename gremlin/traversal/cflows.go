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

package traversal

// #cgo CFLAGS: -I../../probe/ebpf
// #include "flow.h"
import "C"
import (
	"math/rand"
	"net"
	"time"
	"unsafe"

	"github.com/skydive-project/skydive/flow"
)

func newEBPFFlow(id uint32, nodeTID string, linkSrc string, linkDst string) *flow.EBPFFlow {
	ebpfFlow := new(flow.EBPFFlow)
	kernFlow := new(C.struct_flow)
	flow.SetEBPFFlow(ebpfFlow, time.Now(), time.Now(), unsafe.Pointer(kernFlow), 0, nodeTID)
	kernFlow.layers_info |= (1 << 3)
	kernFlow.icmp_layer.id = (C.__u32)(id)
	kernFlow.key = (C.__u64)(rand.Int())
	if linkSrc == "" && linkDst == "" {
		return ebpfFlow
	}
	l2 := C.struct_link_layer{}
	l2A, _ := net.ParseMAC(linkSrc)
	l2B, _ := net.ParseMAC(linkDst)
	for i := 0; i < 6 && i < len(l2A); i++ {
		l2.mac_src[i] = (C.__u8)(l2A[i])
	}
	for i := 0; i < 6 && i < len(l2B); i++ {
		l2.mac_dst[i] = (C.__u8)(l2B[i])
	}
	kernFlow.layers_info |= (1 << 0)
	kernFlow.link_layer = l2
	return ebpfFlow
}
