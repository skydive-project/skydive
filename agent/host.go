/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package agent

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"

	"github.com/dselans/dmidecode"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/topology/graph"
)

// CPUInfo defines host information
type CPUInfo struct {
	CPU        int64
	VendorID   string `json:"VendorID,omitempty"`
	Family     string `json:"Family,omitempty"`
	Model      string `json:"Model,omitempty"`
	Stepping   int64  `json:"Stepping,omitempty"`
	PhysicalID string `json:"PhysicalID,omitempty"`
	CoreID     string `json:"CoreID,omitempty"`
	Cores      int64  `json:"Cores,omitempty"`
	ModelName  string `json:"ModelName,omitempty"`
	Mhz        int64  `json:"Mhz,omitempty"`
	CacheSize  int64  `json:"CacheSize,omitempty"`
	Microcode  string `json:"Microcode,omitempty"`
}

// createRootNode creates a graph.Node based on the host properties and aims to have an unique ID
func createRootNode(g *graph.Graph) (*graph.Node, error) {
	hostID := config.GetString("host_id")
	hostname, err := os.Hostname()
	if err != nil {
		return nil, err
	}
	m := graph.Metadata{"Name": hostID, "Type": "host", "Hostname": hostname}

	// Fill the metadata from the configuration file
	if configMetadata := config.Get("agent.metadata"); configMetadata != nil {
		configMetadata, ok := common.NormalizeValue(configMetadata).(map[string]interface{})
		if !ok {
			return nil, errors.New("agent.metadata has wrong format")
		}
		for k, v := range configMetadata {
			m[k] = v
		}
	}

	// Retrieves the instance ID from cloud-init
	if buffer, err := ioutil.ReadFile("/var/lib/cloud/data/instance-id"); err == nil {
		m.SetField("InstanceID", strings.TrimSpace(string(buffer)))
	}

	if isolated, err := getIsolatedCPUs(); err == nil {
		m.SetField("IsolatedCPU", isolated)
	}

	cpuInfo, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	var cpus []*CPUInfo
	for _, cpu := range cpuInfo {
		c := &CPUInfo{
			CPU:        int64(cpu.CPU),
			VendorID:   cpu.VendorID,
			Family:     cpu.Family,
			Model:      cpu.Model,
			Stepping:   int64(cpu.Stepping),
			PhysicalID: cpu.PhysicalID,
			CoreID:     cpu.CoreID,
			Cores:      int64(cpu.Cores),
			ModelName:  cpu.ModelName,
			Mhz:        int64(cpu.Mhz),
			CacheSize:  int64(cpu.CacheSize),
			Microcode:  cpu.Microcode,
		}
		cpus = append(cpus, c)
	}

	m.SetField("CPU", cpus)

	hostInfo, err := host.Info()
	if err != nil {
		return nil, err
	}

	if hostInfo.OS != "" {
		m.SetField("OS", hostInfo.OS)
	}
	if hostInfo.Platform != "" {
		m.SetField("Platform", hostInfo.Platform)
	}
	if hostInfo.PlatformFamily != "" {
		m.SetField("PlatformFamily", hostInfo.PlatformFamily)
	}
	if hostInfo.PlatformVersion != "" {
		m.SetField("PlatformVersion", hostInfo.PlatformVersion)
	}
	if hostInfo.KernelVersion != "" {
		m.SetField("KernelVersion", hostInfo.KernelVersion)
	}
	if hostInfo.VirtualizationSystem != "" {
		m.SetField("VirtualizationSystem", hostInfo.VirtualizationSystem)
	}
	if hostInfo.VirtualizationRole != "" {
		m.SetField("VirtualizationRole", hostInfo.VirtualizationRole)
	}

	dmi := dmidecode.New()
	if err := dmi.Run(); err == nil {
		// If we have a valid DMI table
		dmi_data := map[string]interface{}{}

		// Let's scan every DMI type
		for type_number := 0; type_number < 128; type_number++ {
			// For each DMI type
			byTypeData, byTypeErr := dmi.SearchByType(type_number)

			// Only consider the populated types
			if byTypeErr == nil {
				// For each entry of that type
				for _, typeData := range byTypeData {
					// The structure will represent that type
					dmi_data_array := []map[string]interface{}{}

					// For every entry of that type
					for dmi_label, dmi_value := range typeData {
						// Some value are sub fields like the Characteristics in Type 0
						// Values are splitted by tabulations
						dmi_value_array := strings.Split(dmi_value, string(9)+string(9))

						if len(dmi_value_array) > 1 {
							// Extract all sub fields if any
							// Compute the sub dynamic structure label : value
							dmi_data_array = append(dmi_data_array, map[string]interface{}{
								dmi_label: dmi_value_array,
							})
						} else {
							// Compute the dynamic structure like label : value
							dmi_data_array = append(dmi_data_array, map[string]interface{}{
								dmi_label: dmi_value,
							})
						}
					}
					// Add the computed value for the given type
					//DMIName represent the human readable name of the current type
					// Like "Bios Information" for type 0
					dmi_data[typeData["DMIName"]] = dmi_data_array
				}
				// Adding all types to the main DMI item
				m.SetField("DMI", dmi_data)
			}
		}
	}

	return g.NewNode(graph.GenID(), m), nil
}
