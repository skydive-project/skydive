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

package common

import (
	"math/rand"
	"time"
)

// Retry tries to execute the given function until a success applying a delay
// between each try
func Retry(fnc func() error, try int, delay time.Duration) error {
	return retry(fnc, try, delay, 1)
}

// RetryExponential tries to execute the given function until a success applying a delay
// between each try. The delay is doubled after each try. Its initial value is baseDelay.
func RetryExponential(fnc func() error, try int, baseDelay time.Duration) error {
	return retry(fnc, try, baseDelay, 2)
}

func retry(fnc func() error, try int, baseDelay time.Duration, factor int64) error {
	var err error
	delay := baseDelay
	for i := 0; i < try; i++ {
		if err = fnc(); err == nil {
			return nil
		}
		time.Sleep(delay)
		delay = time.Duration(factor * int64(delay))
	}
	return err
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// RandString generates random string
func RandString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}
