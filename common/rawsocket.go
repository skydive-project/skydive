// +build linux

/*
 * Copyright (C) 2017 Red Hat, Inc.
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

/*
#ifdef __linux__
#define _GNU_SOURCE
#define __USE_GNU
#include <sched.h>
#include <string.h>
#include <stdlib.h>
#include <sys/socket.h>
#include <linux/sched.h>
#include <linux/if_packet.h>
#include <net/ethernet.h>
#include <arpa/inet.h>
#include <unistd.h>
#include <net/if.h>

int open_raw_socket(const char *name, uint16_t protocol)
{
  struct sockaddr_ll sll;
  int fd;

  fd = socket(PF_PACKET, SOCK_RAW | SOCK_NONBLOCK | SOCK_CLOEXEC,
    htons(ETH_P_ALL));
  if (fd < 0)
    return 0;

  memset(&sll, 0, sizeof(sll));
  sll.sll_family = AF_PACKET;
  sll.sll_ifindex = if_nametoindex(name);
  sll.sll_protocol = htons(protocol);
  if (bind(fd, (struct sockaddr *)&sll, sizeof(sll)) < 0) {
    close(fd);
    return 0;
  }

  return fd;
}

int open_raw_socket_in_netns(int curns, int newns, const char *name, uint16_t protocol)
{
  int errno = 0;
  int fd = 0;

  errno = setns(newns, CLONE_NEWNET);
  if (errno) {
    return 0;
  }

  fd = open_raw_socket(name, protocol);

  errno = setns(curns, CLONE_NEWNET);
  if (errno) {
    if (fd) {
      close(fd);
    }
    return 0;
  }

  return fd;
}
#endif
*/
import "C"

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/vishvananda/netns"
)

// Protocols to receive
const (
	AllPackets    = syscall.ETH_P_ALL
	OnlyIPPackets = syscall.ETH_P_IP
)

// RawSocket describes a raw socket C implemenation
type RawSocket struct {
	fd int
}

// GetFd returns the file descriptor
func (s *RawSocket) GetFd() int {
	return s.fd
}

// Write outputs some bytes to the file
func (s *RawSocket) Write(data []byte) (int, error) {
	return syscall.Write(s.fd, data)
}

// Close the file descriptor
func (s *RawSocket) Close() error {
	if s.fd != 0 {
		_, _, err := syscall.RawSyscall(syscall.SYS_CLOSE, uintptr(s.fd), 0, 0)
		if err != 0 {
			return syscall.Errno(err)
		}
	}
	return nil
}

// NewRawSocket creates a raw socket for the network interface ifName
func NewRawSocket(ifName string, protocol int) (*RawSocket, error) {
	li := unsafe.Pointer(C.CString(ifName))
	defer C.free(li)

	fd := C.open_raw_socket((*C.char)(li), C.uint16_t(protocol))
	if fd == 0 {
		return nil, fmt.Errorf("Failed to open raw socket for %s", ifName)
	}

	return &RawSocket{
		fd: int(fd),
	}, nil
}

// NewRawSocketInNs create/open a socket in the namespace nsPath for the network interface ifName
func NewRawSocketInNs(nsPath string, ifName string, protocol int) (*RawSocket, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	origns, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("Error while getting current ns: %s", err.Error())
	}
	defer origns.Close()

	newns, err := netns.GetFromPath(nsPath)
	if err != nil {
		return nil, fmt.Errorf("Error while opening %s: %s", nsPath, err.Error())
	}
	defer newns.Close()

	pIfName := unsafe.Pointer(C.CString(ifName))
	defer C.free(pIfName)

	fd := C.open_raw_socket_in_netns(C.int(origns), C.int(newns), (*C.char)(pIfName), C.uint16_t(protocol))
	if fd == 0 {
		return nil, fmt.Errorf("Failed to open raw socket for %s", ifName)
	}

	return &RawSocket{
		fd: int(fd),
	}, nil
}
