// +build linux

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

package agent

import (
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"
)

func parseIsolatedCPUs(str string) ([]int64, error) {
	list := strings.Split(str, ",")

	var isolated []int64
	for _, el := range list {
		// range
		if strings.Contains(el, "-") {
			rg := strings.Split(el, "-")
			if len(rg) != 2 {
				return nil, fmt.Errorf("Range error: %s", el)
			}
			start, err := strconv.Atoi(rg[0])
			if err != nil {
				return nil, fmt.Errorf("Range error: %s", el)
			}
			end, err := strconv.Atoi(rg[1])
			if err != nil {
				return nil, fmt.Errorf("Range error: %s", el)
			}

			if start > end {
				return nil, fmt.Errorf("Range error: %s", el)
			}

			for i := start; i <= end; i++ {
				isolated = append(isolated, int64(i))
			}
		} else {
			v, err := strconv.Atoi(el)
			if err != nil {
				return nil, fmt.Errorf("Value error: %s", el)
			}

			isolated = append(isolated, int64(v))
		}
	}

	return isolated, nil
}

func getIsolatedCPUs() ([]int64, error) {
	buffer, err := ioutil.ReadFile("/sys/devices/system/cpu/isolated")
	if err != nil {
		return nil, err
	}

	return parseIsolatedCPUs(strings.TrimSpace(string(buffer)))
}
