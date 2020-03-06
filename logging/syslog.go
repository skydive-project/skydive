// +build !windows

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

package logging

import (
	"log/syslog"

	"github.com/tchap/zapext/zapsyslog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type syslogBackend struct {
	w *syslog.Writer
}

func (b *syslogBackend) Core(msgPriority zap.LevelEnablerFunc, encoder zapcore.Encoder) zapcore.Core {
	return zapsyslog.NewCore(msgPriority, encoder, b.w)
}

// NewSyslogBackend returns a new backend that outputs to syslog
func NewSyslogBackend(protocol, address, tag string) (Backend, error) {
	w, err := syslog.Dial(protocol, address, syslog.LOG_EMERG|syslog.LOG_KERN, tag)
	if err != nil {
		return nil, err
	}
	return &syslogBackend{w: w}, nil
}
