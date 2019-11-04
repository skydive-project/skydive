// +build linux,!ppc64le

/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package lldp

import (
	"net"
	"syscall"
)

func rawSocketaddrFromMAC(mac net.HardwareAddr) (sockaddr syscall.RawSockaddr) {
	for i, n := range mac {
		sockaddr.Data[i] = int8(n)
	}
	return
}
