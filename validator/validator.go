/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package validator

import (
	"errors"
	"regexp"

	"gopkg.in/validator.v2"
)

var skydiveValidator = validator.NewValidator()

func isIP(v interface{}, param string) error {
	//(TODO: masco) need to support IPv6 also
	ipv4Regex := "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5]).){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
	ipError := validator.TextErr{errors.New("Not a IP addr")}
	ip, ok := v.(string)
	if !ok {
		return ipError
	}
	re, _ := regexp.Compile(ipv4Regex)
	if !re.MatchString(ip) {
		return ipError
	}
	return nil
}

func Validate(v interface{}) error {
	return skydiveValidator.Validate(v)
}

func init() {
	skydiveValidator.SetValidationFunc("isIP", isIP)
	skydiveValidator.SetTag("valid")
}
