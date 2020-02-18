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

package filters

import (
	fmt "fmt"
	math "math"
	"net"
	"sort"
	"strconv"
	"strings"
)

// RangeToRegex returns a regular expression matching number in the given range
// Golang version of https://github.com/dimka665/range-regex
func RangeToRegex(min, max int) string {
	subpatterns := splitToPatterns(min, max)
	return strings.Join(subpatterns, "|")
}

// IPV4CIDRToRegex returns a regex matching IPs belonging to a given cidr
func IPV4CIDRToRegex(cidr string) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	netIP := ipnet.IP.To4()
	firstIP := netIP.Mask(ipnet.Mask)
	lastIP := net.IPv4(0, 0, 0, 0).To4()

	var regex string
	var groupMask bool
	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^ipnet.Mask[i]

		fip := int(firstIP[i])
		lip := int(lastIP[i])

		if regex != "" {
			regex += `\.`
		}

		if fip == lip {
			if !groupMask {
				regex += "("
				groupMask = true
			}

			regex += strconv.Itoa(fip)
		} else {
			if groupMask {
				regex += ")"
				groupMask = false
			}

			regex += "(" + RangeToRegex(fip, lip) + ")"
		}
	}

	if groupMask {
		regex += ")"
	}
	return "^" + regex + `(\/[0-9]?[0-9])?$`, nil
}

func splitToRanges(min, max int) []int {
	stops := map[int]bool{max: true}

	ninesCount := 1
	stop := fillByNines(min, ninesCount)
	for min <= stop && stop < max {
		stops[stop] = true

		ninesCount++
		stop = fillByNines(min, ninesCount)
	}

	zerosCount := 1
	stop = fillByZeros(max+1, zerosCount) - 1
	for min < stop && stop <= max {
		stops[stop] = true

		zerosCount++
		stop = fillByZeros(max+1, zerosCount) - 1
	}

	var sr []int
	for i := range stops {
		sr = append(sr, i)
	}

	sort.Ints(sr)

	return sr
}

func rangeToPattern(start, stop int) string {
	pattern := ""
	anyDigitCount := 0

	startStr, stopStr := strconv.Itoa(start), strconv.Itoa(stop)
	for i := 0; i != len(startStr); i++ {
		startDigit, stopDigit := string(startStr[i]), string(stopStr[i])

		if startDigit == stopDigit {
			pattern += startDigit
		} else if startDigit != "0" || stopDigit != "9" {
			pattern += fmt.Sprintf("[%s-%s]", startDigit, stopDigit)
		} else {
			anyDigitCount++
		}
	}

	if anyDigitCount > 0 {
		pattern += "[0-9]"
	}

	if anyDigitCount > 1 {
		pattern += fmt.Sprintf("{%d}", anyDigitCount)
	}

	return pattern
}

func splitToPatterns(min, max int) []string {
	var subpatterns []string

	start := min
	for _, stop := range splitToRanges(min, max) {
		subpatterns = append(subpatterns, rangeToPattern(start, stop))
		start = stop + 1
	}

	return subpatterns
}

func fillByNines(i, count int) int {
	str := strconv.Itoa(i)

	var prefix string
	if count < len(str) {
		prefix = str[:len(str)-count]
	}
	i, _ = strconv.Atoi(prefix + strings.Repeat("9", count))
	return i
}

func fillByZeros(i, count int) int {
	return i - i%int(math.Pow10(count))
}
