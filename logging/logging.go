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
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/op/go-logging"
	"github.com/skydive-project/skydive/config"
)

func getPackageFunction() (pkg string, fun string) {
	pkg, fun = "???", "???"
	if pc, _, _, ok := runtime.Caller(2); ok {
		if fr := runtime.FuncForPC(pc); fr != nil {
			f := fr.Name()
			i := strings.LastIndex(f, "/")
			j := strings.Index(f[i+1:], ".")
			if j < 1 {
				return "???", "???"
			}
			pkg, fun = f[:i+j+1], f[i+j+2:]
		}
	}
	return pkg, fun
}

var skydiveLoggerLock sync.Mutex
var skydiveLogger SkydiveLogger
var skydiveLoggingID = "skydive"

func SetLoggingID(ID string) {
	skydiveLoggingID = ID
}

type SkydiveLogger struct {
	loggers     map[string]*logging.Logger
	id          string
	format      string
	formatDebug string
	backend     logging.Backend
}

func initSkydiveLogger() {
	id := config.GetConfig().GetString("host_id") + ":" + skydiveLoggingID
	skydiveLogger = SkydiveLogger{
		id:          id,
		loggers:     make(map[string]*logging.Logger),
		format:      "%{color}%{time} " + id + " %{shortfile} %{shortpkg} %{longfunc} > %{level:.4s} %{id:03x}%{color:reset} %{message}",
		formatDebug: "%{color}%{time} " + id + " %{shortfile} %{shortpkg} %{callpath:5} %{longfunc} > %{level:.4s} %{id:03x}%{color:reset} %{message}",
	}
	newLogger("default", "INFO")
}

func newLogger(pkg string, loglevel string) error {
	level, err := logging.LogLevel(loglevel)
	if err != nil {
		return err
	}
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	format := skydiveLogger.format
	if level == logging.DEBUG {
		format = skydiveLogger.formatDebug
	}
	backendFormat := logging.NewBackendFormatter(backend, logging.MustStringFormatter(format))
	backendLevel := logging.AddModuleLevel(backendFormat)
	backendLevel.SetLevel(level, pkg)

	logger, err := logging.GetLogger(pkg)
	if err != nil {
		return err
	}
	logger.SetBackend(backendLevel)
	skydiveLogger.loggers[pkg] = logger

	skydiveLogger.loggers["default"].Debug("New Log Registered : " + pkg + " " + loglevel)
	return nil
}

func InitLogger() error {
	skydiveLoggerLock.Lock()
	defer skydiveLoggerLock.Unlock()
	return initLogger()
}

func initLogger() (err error) {
	initSkydiveLogger()
	cfg := config.GetConfig()

	for cfgPkg, cfgLvl := range cfg.GetStringMapString("logging") {
		pkg := strings.TrimSpace(cfgPkg)
		lvl := strings.TrimSpace(cfgLvl)
		if pkg == "default" {
			err = newLogger("default", lvl)
		} else {
			err = newLogger("github.com/skydive-project/skydive/"+pkg, lvl)
		}
		if err != nil {
			return errors.New("Can't parse logging line : \"" + pkg + " " + lvl + "\" " + err.Error())
		}
	}
	return
}

func GetLogger() (log *logging.Logger) {
	skydiveLoggerLock.Lock()
	defer skydiveLoggerLock.Unlock()

	pkg, f := getPackageFunction()
	log, found := skydiveLogger.loggers[pkg+"."+f]
	if !found {
		log, found = skydiveLogger.loggers[pkg]
		if !found {
			log, found = skydiveLogger.loggers["default"]
			if !found {
				err := initLogger()
				if err != nil {
					fmt.Fprintf(os.Stderr, "%v\n", err)
					os.Exit(1)
				}
				log, _ = skydiveLogger.loggers["default"]
			}
		}
	}
	return log
}
