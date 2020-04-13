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
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrCantCompareInterface error can't compare interface
	ErrCantCompareInterface = errors.New("Can't compare interface")
	// ErrFieldWrongType error field has wrong type
	ErrFieldWrongType = errors.New("Field has wrong type")
	// ErrNotFound error no result was found
	ErrNotFound = errors.New("No result found")
	// ErrTimeout network timeout
	ErrTimeout = errors.New("Timeout")
	// ErrNotImplemented unimplemented feature
	ErrNotImplemented = errors.New("Not implemented")
)

// SortOrder describes ascending or descending order
type SortOrder string

const (
	// SortAscending sorting order
	SortAscending SortOrder = "ASC"
	// SortDescending sorting order
	SortDescending SortOrder = "DESC"
)

// UnixMillis returns the current time in miliseconds
func UnixMillis(t time.Time) int64 {
	return t.UTC().UnixNano() / 1000000
}

// TimeSlice defines a time boudary values
type TimeSlice struct {
	Start int64 `json:"Start"`
	Last  int64 `json:"Last"`
}

// NewTimeSlice creates a new TimeSlice based on Start and Last
func NewTimeSlice(s, l int64) *TimeSlice {
	return &TimeSlice{Start: s, Last: l}
}

// Metric defines a common metric interface
type Metric interface {
	// part of the Getter interface
	GetFieldInt64(field string) (int64, error)
	GetFieldKeys() []string

	Add(m Metric) Metric
	Sub(m Metric) Metric
	Split(cut int64) (Metric, Metric)
	GetStart() int64
	SetStart(start int64)
	GetLast() int64
	SetLast(last int64)
	IsZero() bool
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

// IPStrToUint32 converts IP string to 32bits
func IPStrToUint32(ipAddr string) (uint32, error) {
	ip := net.ParseIP(ipAddr)
	if ip == nil {
		return 0, errors.New("wrong ipAddr format")
	}
	ip = ip.To4()
	if ip == nil {
		return 0, errors.New("wrong ipAddr format")
	}
	return binary.BigEndian.Uint32(ip), nil
}
