// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// +build !linux

package afpacket

import (
	"time"
)

type OptInterface string
type OptFrameSize int
type OptBlockSize int
type OptNumBlocks int
type OptBlockTimeout time.Duration
type OptPollTimeout time.Duration

// Default constants used by options.
const (
	DefaultFrameSize    = 4096                   // Default value for OptFrameSize.
	DefaultBlockSize    = DefaultFrameSize * 128 // Default value for OptBlockSize.
	DefaultNumBlocks    = 128                    // Default value for OptNumBlocks.
	DefaultBlockTimeout = 64 * time.Millisecond  // Default value for OptBlockTimeout.
	DefaultPollTimeout  = -1 * time.Millisecond  // Default value for OptPollTimeout. This blocks forever.
)
