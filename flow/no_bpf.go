// +build !linux

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

package flow

import (
	"github.com/google/gopacket/layers"
	"golang.org/x/net/bpf"

	"github.com/skydive-project/skydive/common"
)

// BPF describes a filter
type BPF struct {
	vm *bpf.VM
}

// BPFFilterToRaw creates a raw binary filter from a BPF expression
func BPFFilterToRaw(linkType layers.LinkType, captureLength uint32, filter string) ([]bpf.RawInstruction, error) {
	return nil, common.ErrNotImplemented
}

// Matches returns true data match the filter
func (b *BPF) Matches(data []byte) bool {
	return false
}

// NewBPF creates a new BPF filter
func NewBPF(linkType layers.LinkType, captureLength uint32, filter string) (*BPF, error) {
	return nil, common.ErrNotImplemented
}
