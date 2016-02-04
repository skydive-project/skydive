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

package config

import (
	"errors"
	"fmt"
	"strconv"

	"gopkg.in/ini.v1"
)

var cfg *ini.File

func InitConfigFromFile(filename string) error {
	var err error

	cfg, err = ini.Load(filename)
	if err != nil {
		return err
	}

	return nil
}

func GetConfig() *ini.File {
	return cfg
}

func GetHostPortAttributes(s string, p string) (string, int, error) {
	listen := GetConfig().Section(s).Key(p).Strings(":")

	addr := "127.0.0.1"

	switch l := len(listen); {
	case l == 1:
		port, err := strconv.Atoi(listen[0])
		if err != nil {
			return "", 0, err
		}

		return addr, port, nil
	case l == 2:
		port, err := strconv.Atoi(listen[1])
		if err != nil {
			return "", 0, err
		}

		return listen[0], port, nil
	default:
		return "", 0, errors.New(fmt.Sprintf("Malformed listen parameter %s in section %s", s, p))
	}
}
