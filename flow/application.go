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

package flow

import (
	"strconv"
	"strings"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/logging"
)

// ApplicationPortMap maps UDP and TCP port numbers to service names
type ApplicationPortMap struct {
	UDP map[int]string
	TCP map[int]string
}

func (a *ApplicationPortMap) application(srcPort, dstPort int, protoMap map[int]string) (string, bool) {
	if app, ok := protoMap[srcPort]; ok {
		return app, ok
	}
	if app, ok := protoMap[dstPort]; ok {
		return app, ok
	}

	return "", false
}

func (a *ApplicationPortMap) tcpApplication(srcPort, dstPort int) (string, bool) {
	if a == nil {
		return "", false
	}
	return a.application(srcPort, dstPort, a.TCP)
}

func (a *ApplicationPortMap) udpApplication(srcPort, dstPort int) (string, bool) {
	if a == nil {
		return "", false
	}
	return a.application(srcPort, dstPort, a.UDP)
}

func (a *ApplicationPortMap) init() {
	for _, protoName := range []string{"udp", "tcp"} {

		m := config.GetStringMapString("flow.application_ports." + protoName)
		for port, name := range m {
			i, err := strconv.Atoi(port)
			if err != nil {
				logging.GetLogger().Errorf("Unable to parse %s application port %s: %s", protoName, port, err)
				continue
			}
			name = strings.ToUpper(name)

			switch protoName {
			case "udp":
				a.UDP[i] = name
			case "tcp":
				a.TCP[i] = name
			}
		}
	}
}

// NewApplicationPortMapFromConfig returns a new application port mapper
// and load it from the configuration file
func NewApplicationPortMapFromConfig() *ApplicationPortMap {
	apm := &ApplicationPortMap{
		TCP: make(map[int]string),
		UDP: make(map[int]string),
	}
	apm.init()

	return apm
}
