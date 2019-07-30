// +build collectd

/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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

package main

import (
	"fmt"
	"os"
	"strings"
	"unsafe"
)

/*
#cgo CFLAGS:
#cgo LDFLAGS: -Wl,-unresolved-symbols=ignore-all

#include <stdio.h>

#include "plugin.h"

static void log_msg(int level, char *msg) {
	plugin_log(level, msg);
}
*/
import "C"

type logger struct{}

// Logger logger using collectd helper
var Logger logger

func (l *logger) logf(level C.int, format *string, args ...interface{}) {
	var msg string

	f := "skydive plugin - "
	if format == nil {
		f += strings.Repeat(" %v", len(args))
	} else {
		f += *format
	}
	msg = fmt.Sprintf(f, args...)

	var cmsg *C.char = C.CString(msg)
	defer C.free(unsafe.Pointer(cmsg))

	C.log_msg(C.LOG_ERR, cmsg)
}

// Fatal log level
func (l *logger) Fatal(args ...interface{}) {
	l.logf(C.LOG_ERR, nil, args...)
	os.Exit(1)
}

// Fatalf log level
func (l *logger) Fatalf(format string, args ...interface{}) {
	l.logf(C.LOG_ERR, &format, args...)
	os.Exit(1)
}

// Panic log level
func (l *logger) Panic(args ...interface{}) {
	l.logf(C.LOG_ERR, nil, args...)
}

// Panicf log level
func (l *logger) Panicf(format string, args ...interface{}) {
	l.logf(C.LOG_ERR, &format, args...)
}

// Critical log level
func (l *logger) Critical(args ...interface{}) {
	l.logf(C.LOG_ERR, nil, args...)
}

// Criticalf log level
func (l *logger) Criticalf(format string, args ...interface{}) {
	l.logf(C.LOG_ERR, &format, args...)
}

// Error log level
func (l *logger) Error(args ...interface{}) {
	l.logf(C.LOG_ERR, nil, args...)
}

// Errorf log level
func (l *logger) Errorf(format string, args ...interface{}) {
	l.logf(C.LOG_ERR, &format, args...)
}

// Warning log level
func (l *logger) Warning(args ...interface{}) {
	l.logf(C.LOG_WARNING, nil, args...)
}

// Warningf log level
func (l *logger) Warningf(format string, args ...interface{}) {
	l.logf(C.LOG_WARNING, &format, args...)
}

// Notice log level
func (l *logger) Notice(args ...interface{}) {
	l.logf(C.LOG_NOTICE, nil, args...)
}

// Noticef log level
func (l *logger) Noticef(format string, args ...interface{}) {
	l.logf(C.LOG_NOTICE, &format, args...)
}

// Info log level
func (l *logger) Info(args ...interface{}) {
	l.logf(C.LOG_INFO, nil, args...)
}

// Infof log level
func (l *logger) Infof(format string, args ...interface{}) {
	l.logf(C.LOG_INFO, &format, args...)
}

// Debug log level
func (l *logger) Debug(args ...interface{}) {
	l.logf(C.LOG_DEBUG, nil, args...)
}

// Debugf log level
func (l *logger) Debugf(format string, args ...interface{}) {
	l.logf(C.LOG_DEBUG, &format, args...)
}
