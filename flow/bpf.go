/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package flow

import (
	"fmt"

	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"golang.org/x/net/bpf"
)

// BPF describes a filter
type BPF struct {
	vm *bpf.VM
}

// BPFFilterToRaw creates a raw binary filter from a BPF expression
func BPFFilterToRaw(linkType layers.LinkType, captureLength uint32, filter string) ([]bpf.RawInstruction, error) {
	// use pcap bpf compiler to get raw bpf instruction
	pcapBPF, err := pcap.CompileBPFFilter(linkType, int(captureLength), filter)
	if err != nil {
		return nil, err
	}
	rawBPF := make([]bpf.RawInstruction, len(pcapBPF))
	for i, ri := range pcapBPF {
		rawBPF[i] = bpf.RawInstruction{Op: ri.Code, Jt: ri.Jt, Jf: ri.Jf, K: ri.K}
	}

	return rawBPF, nil
}

// Matches returns true data match the filter
func (b *BPF) Matches(data []byte) bool {
	if b.vm == nil {
		return true
	}

	if n, err := b.vm.Run(data); err != nil || n == 0 {
		return false
	}

	return true
}

// NewBPF creates a new BPF filter
func NewBPF(linkType layers.LinkType, captureLength uint32, filter string) (*BPF, error) {
	if filter == "" {
		return &BPF{}, nil
	}

	rawBPF, err := BPFFilterToRaw(linkType, captureLength, filter)
	if err != nil {
		return nil, err
	}

	instBPF, ok := bpf.Disassemble(rawBPF)
	if !ok {
		return nil, fmt.Errorf("BPF expression not correctly decoded: %s", filter)
	}

	vm, err := bpf.NewVM(instBPF)
	if err != nil {
		return nil, err
	}

	return &BPF{
		vm: vm,
	}, nil
}
