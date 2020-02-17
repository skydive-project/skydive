/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package common

import (
	"fmt"
	"strings"
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

// ParseAddr parses an address of the form protocol://target such as
// unix:////var/run/service/program.sock or tcp://my.domain:2134. It also
// handles addresses of the form address:port and assumes it uses TCP.
func ParseAddr(address string) (protocol string, target string, err error) {
	if strings.HasPrefix(address, "unix://") {
		target = strings.TrimPrefix(address, "unix://")
		protocol = "unix"
	} else if strings.HasPrefix(address, "tcp://") {
		target = strings.TrimPrefix(address, "tcp://")
		protocol = "tcp"
	} else {
		// fallback to the original address format addr:port
		sa, err := ServiceAddressFromString(address)
		if err != nil {
			return "", "", err
		}
		protocol = "tcp"
		target = fmt.Sprintf("%s:%d", sa.Addr, sa.Port)
	}
	return
}
