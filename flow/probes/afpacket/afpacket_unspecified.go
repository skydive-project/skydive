// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// +build !linux

// Package afpacket provides Go bindings for MMap'd AF_PACKET socket reading.
package afpacket

import (
	"errors"

	"golang.org/x/net/bpf"

	"github.com/google/gopacket"
)

var (
	ErrNotImplemented = errors.New("not implemented")
	ErrTimeout        = errors.New("packet poll timeout expired")
)

type TPacket struct {
}

type SocketStats struct{}
type SocketStatsV3 struct{}

func (h *TPacket) ReadPacketData() (data []byte, ci gopacket.CaptureInfo, err error) {
	return []byte{}, gopacket.CaptureInfo{}, ErrNotImplemented
}

func (h *TPacket) Close() {
}

func NewTPacket(opts ...interface{}) (h *TPacket, err error) {
	return nil, ErrNotImplemented
}

func (h *TPacket) SetBPF(filter []bpf.RawInstruction) error {
	return ErrNotImplemented
}

// SocketStats saves stats from the socket to the TPacket instance.
func (h *TPacket) SocketStats() (SocketStats, SocketStatsV3, error) {
	return SocketStats{}, SocketStatsV3{}, ErrNotImplemented
}

func (s SocketStatsV3) Packets() uint64 {
	return 0
}

func (s SocketStatsV3) Drops() uint64 {
	return 0
}
