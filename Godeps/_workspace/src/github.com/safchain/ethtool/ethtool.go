/*
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

// Package ethtool  aims to provide a library giving a simple access to the
// Linux SIOCETHTOOL ioctl operations. It can be used to retrieve informations
// from a network device like statistics, driver related informations or
// even the peer of a VETH interface.
package ethtool

import (
	"bytes"
	"fmt"
	"syscall"
	"unsafe"
)

// Maximum size of an interface name
const (
	IFNAMSIZ = 16
)

// ioctl ethtool request
const (
	SIOCETHTOOL = 0x8946
)

// ethtool stats related constants.
const (
	ETH_GSTRING_LEN  = 32
	ETH_SS_STATS     = 1
	ETHTOOL_GDRVINFO = 0x00000003
	ETHTOOL_GSTRINGS = 0x0000001b
	ETHTOOL_GSTATS   = 0x0000001d
)

// MAX_GSTRINGS maximum number of stats entries that ethtool can
// retrieve currently.
const (
	MAX_GSTRINGS = 100
)

type ifreq struct {
	ifr_name [IFNAMSIZ]byte
	ifr_data uintptr
}

type ethtoolDrvInfo struct {
	cmd          uint32
	driver       [32]byte
	version      [32]byte
	fw_version   [32]byte
	bus_info     [32]byte
	erom_version [32]byte
	reserved2    [12]byte
	n_priv_flags uint32
	n_stats      uint32
	testinfo_len uint32
	eedump_len   uint32
	regdump_len  uint32
}

type ethtoolGStrings struct {
	cmd        uint32
	string_set uint32
	len        uint32
	data       [MAX_GSTRINGS * ETH_GSTRING_LEN]byte
}

type ethtoolStats struct {
	cmd     uint32
	n_stats uint32
	data    [MAX_GSTRINGS]uint64
}

// DriverName returns the driver name of the given interface.
func DriverName(intf string) (string, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_IP)
	if err != nil {
		return "", err
	}
	defer syscall.Close(fd)

	drvinfo := ethtoolDrvInfo{
		cmd: ETHTOOL_GDRVINFO,
	}

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(&drvinfo)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return "", syscall.Errno(ep)
	}

	return string(bytes.Trim(drvinfo.driver[:], "\x00")), nil
}

// Stats retrieves stats of the given interface name.
func Stats(intf string) (map[string]uint64, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_IP)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	drvinfo := ethtoolDrvInfo{
		cmd: ETHTOOL_GDRVINFO,
	}

	var name [IFNAMSIZ]byte
	copy(name[:], []byte(intf))

	ifr := ifreq{
		ifr_name: name,
		ifr_data: uintptr(unsafe.Pointer(&drvinfo)),
	}

	_, _, ep := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return nil, syscall.Errno(ep)
	}

	if drvinfo.n_stats*ETH_GSTRING_LEN > MAX_GSTRINGS*ETH_GSTRING_LEN {
		return nil, fmt.Errorf("ethtool currently doesn't support more than %d entries, received %d", MAX_GSTRINGS, drvinfo.n_stats)
	}

	gstrings := ethtoolGStrings{
		cmd:        ETHTOOL_GSTRINGS,
		string_set: ETH_SS_STATS,
		len:        drvinfo.n_stats,
		data:       [MAX_GSTRINGS * ETH_GSTRING_LEN]byte{},
	}
	ifr.ifr_data = uintptr(unsafe.Pointer(&gstrings))

	_, _, ep = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return nil, syscall.Errno(ep)
	}

	stats := ethtoolStats{
		cmd:     ETHTOOL_GSTATS,
		n_stats: drvinfo.n_stats,
		data:    [MAX_GSTRINGS]uint64{},
	}

	ifr.ifr_data = uintptr(unsafe.Pointer(&stats))

	_, _, ep = syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr)))
	if ep != 0 {
		return nil, syscall.Errno(ep)
	}

	var result = make(map[string]uint64)
	for i := 0; i != int(drvinfo.n_stats); i++ {
		b := gstrings.data[i*ETH_GSTRING_LEN : i*ETH_GSTRING_LEN+ETH_GSTRING_LEN]
		key := string(bytes.Trim(b, "\x00"))
		result[key] = stats.data[i]
	}

	return result, nil
}
