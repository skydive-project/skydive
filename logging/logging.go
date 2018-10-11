/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package logging

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger describes an identifier logger
type Logger struct {
	*zap.Logger
	id string
}

var currentLogger *Logger

type level int

// Logging levels
const (
	CRITICAL level = iota
	ERROR
	WARNING
	NOTICE
	INFO
	DEBUG
)

func (l *Logger) log(level level, format *string, args ...interface{}) {
	s := l.Sugar()
	fmt := l.id
	if format != nil {
		fmt += " " + *format
	} else {
		for range args {
			fmt += " %v"
		}
	}

	switch level {
	case CRITICAL:
		s.DPanicf(fmt, args...)
	case ERROR:
		s.Errorf(fmt, args...)
	case WARNING:
		s.Warnf(fmt, args...)
	case NOTICE:
		s.Infof(fmt, args...)
	case INFO:
		s.Infof(fmt, args...)
	case DEBUG:
		s.Debugf(fmt, args...)
	}
}

func getZapLevel(level string) zapcore.Level {
	lvl := zapcore.DebugLevel
	switch level {
	case "CRITICAL":
		lvl = zapcore.DPanicLevel
	case "ERROR":
		lvl = zapcore.ErrorLevel
	case "WARNING":
		lvl = zapcore.WarnLevel
	case "NOTICE":
		lvl = zapcore.InfoLevel
	case "INFO":
		lvl = zapcore.InfoLevel
	case "DEBUG":
		lvl = zapcore.DebugLevel
	}
	return lvl
}

// Fatal is equivalent to l.Critical(fmt.Sprint()) followed by a call to os.Exit(1).
func (l *Logger) Fatal(args ...interface{}) {
	l.log(CRITICAL, nil, args...)
	os.Exit(1)
}

// Fatalf is equivalent to l.Critical followed by a call to os.Exit(1).
func (l *Logger) Fatalf(format string, args ...interface{}) {
	l.log(CRITICAL, &format, args...)
	os.Exit(1)
}

// Panic is equivalent to l.Critical(fmt.Sprint()) followed by a call to panic().
func (l *Logger) Panic(args ...interface{}) {
	l.log(CRITICAL, nil, args...)
	panic(fmt.Sprint(args...))
}

// Panicf is equivalent to l.Critical followed by a call to panic().
func (l *Logger) Panicf(format string, args ...interface{}) {
	l.log(CRITICAL, &format, args...)
	panic(fmt.Sprintf(format, args...))
}

// Critical logs a message using CRITICAL as log level.
func (l *Logger) Critical(args ...interface{}) {
	l.log(CRITICAL, nil, args...)
}

// Criticalf logs a message using CRITICAL as log level.
func (l *Logger) Criticalf(format string, args ...interface{}) {
	l.log(CRITICAL, &format, args...)
}

// Error logs a message using ERROR as log level.
func (l *Logger) Error(args ...interface{}) {
	l.log(ERROR, nil, args...)
}

// Errorf logs a message using ERROR as log level.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ERROR, &format, args...)
}

// Warning logs a message using WARNING as log level.
func (l *Logger) Warning(args ...interface{}) {
	l.log(WARNING, nil, args...)
}

// Warningf logs a message using WARNING as log level.
func (l *Logger) Warningf(format string, args ...interface{}) {
	l.log(WARNING, &format, args...)
}

// Notice logs a message using NOTICE as log level.
func (l *Logger) Notice(args ...interface{}) {
	l.log(NOTICE, nil, args...)
}

// Noticef logs a message using NOTICE as log level.
func (l *Logger) Noticef(format string, args ...interface{}) {
	l.log(NOTICE, &format, args...)
}

// Info logs a message using INFO as log level.
func (l *Logger) Info(args ...interface{}) {
	l.log(INFO, nil, args...)
}

// Infof logs a message using INFO as log level.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(INFO, &format, args...)
}

// Debug logs a message using DEBUG as log level.
func (l *Logger) Debug(args ...interface{}) {
	l.log(DEBUG, nil, args...)
}

// Debugf logs a message using DEBUG as log level.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DEBUG, &format, args...)
}

func shortCallerWithClassFunctionEncoder(caller zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
	path := caller.TrimmedPath()
	if f := runtime.FuncForPC(caller.PC); f != nil {
		name := f.Name()
		i := strings.LastIndex(name, "/")
		j := strings.Index(name[i+1:], ".")
		path += " " + name[i+j+2:]
	}
	enc.AppendString(path)
}

func newEncoderConfig(color bool) zapcore.EncoderConfig {
	encoder := zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:        "ts",
		LevelKey:       "level",
		NameKey:        "name",
		CallerKey:      "caller",
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalColorLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   shortCallerWithClassFunctionEncoder,
	}
	if !color {
		encoder.EncodeLevel = zapcore.CapitalLevelEncoder
	}
	return encoder
}

func newConfig(encoderConfig zapcore.EncoderConfig) zap.Config {
	return zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      false,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

// Backend describes a logging backend
type Backend interface {
	Core(msgPriority zap.LevelEnablerFunc, encoder zapcore.Encoder) zapcore.Core
}

// LoggerConfig describes a logger configuration such as backend, log level or an encoding
type LoggerConfig struct {
	backend  Backend
	logLevel string
	encoding string
}

// NewLoggerConfig returns a logger configuration with the associated logging backend, log level and encoding
func NewLoggerConfig(backend Backend, logLevel string, encoding string) *LoggerConfig {
	return &LoggerConfig{backend: backend, logLevel: logLevel, encoding: encoding}
}

type fileBackend struct {
	file *os.File
}

// Core returns a zap core
func (b *fileBackend) Core(msgPriority zap.LevelEnablerFunc, encoder zapcore.Encoder) zapcore.Core {
	return zapcore.NewCore(encoder, zapcore.Lock(b.file), msgPriority)
}

// NewFileBackend returns a logging backend that outputs to a file
func NewFileBackend(filename string) (Backend, error) {
	file, err := os.Create(filename)
	if err != nil {
		return nil, err
	}
	return &fileBackend{file: file}, nil
}

// NewStdioBackend returns a logging backend that outputs to the standard output
func NewStdioBackend(file *os.File) Backend {
	return &fileBackend{file: file}
}

// InitLogging initializes logging system with an identifier for the log messages,
// whether the output should be in colored mode and a set of logger configs.
func InitLogging(id string, color bool, loggers []*LoggerConfig) (err error) {
	getMessagePriority := func(backendLevel zapcore.Level) zap.LevelEnablerFunc {
		return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= backendLevel
		})
	}

	encoderConfig := newEncoderConfig(color)
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	jsonEncoder := zapcore.NewJSONEncoder(encoderConfig)

	var cores []zapcore.Core
	for _, logger := range loggers {
		var encoder zapcore.Encoder
		switch logger.encoding {
		case "json":
			encoder = jsonEncoder
		default:
			encoder = consoleEncoder
		}
		backendLevel := getZapLevel(logger.logLevel)
		cores = append(cores, logger.backend.Core(getMessagePriority(backendLevel), encoder))
	}

	newCore := zap.WrapCore(func(zapcore.Core) zapcore.Core {
		return zapcore.NewTee(cores...)
	})

	c := newConfig(encoderConfig)
	z, err := c.Build(
		newCore,
		zap.AddCallerSkip(2),
		// uncomment the following line to get stacktrace on error messages
		// zap.AddStacktrace(zapcore.ErrorLevel),
		zap.AddStacktrace(zapcore.DPanicLevel),
	)
	if err != nil {
		return err
	}

	currentLogger = &Logger{
		Logger: z,
		id:     id,
	}
	return nil
}

// GetLogger returns the current logger instance
func GetLogger() (log *Logger) {
	return currentLogger
}

func init() {
	hostname, _ := os.Hostname()
	InitLogging(hostname, false, []*LoggerConfig{NewLoggerConfig(NewStdioBackend(os.Stderr), "INFO", "")})
}
