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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/mitchellh/mapstructure"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/spf13/viper"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/logging"
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

	if cmdline, err := ioutil.ReadFile("/proc/cmdline"); err == nil {
		kernelArgs := make(map[string]interface{})
		for _, arg := range strings.Split(string(cmdline), " ") {
			if splitted := strings.SplitN(arg, "=", 2); len(splitted) == 1 {
				kernelArgs[strings.TrimSpace(splitted[0])] = true
			} else {
				kernelArgs[strings.TrimSpace(splitted[0])] = strings.TrimSpace(splitted[1])
			}
		}
		m.SetField("KernelCmdLine", kernelArgs)
	}

	hostNode, err := g.NewNode(graph.GenID(), m)
	if err != nil {
		return nil, err
	}

	parseMetadataConfigFiles(g, hostNode)

	return hostNode, nil
}

type metadataConfigFile struct {
	Path     string
	Type     string
	Name     string
	Selector string
}

func parseMetadataConfigFiles(g *graph.Graph, hostNode *graph.Node) error {
	files := config.Get("agent.metadata_config.files")
	if files == nil {
		return nil
	}

	var mcfs []metadataConfigFile
	if err := mapstructure.Decode(files, &mcfs); err != nil {
		return fmt.Errorf("Unable to read agent.metadata_config.files: %s", err)
	}

	updatedHostNode := func(v *viper.Viper, mcf metadataConfigFile) {
		s := v.GetString(mcf.Selector)

		g.Lock()
		g.AddMetadata(hostNode, fmt.Sprintf("Config.%s", mcf.Name), s)
		g.Unlock()
	}

	for _, mcf := range mcfs {
		v := viper.New()
		v.SetConfigFile(mcf.Path)

		typ := mcf.Type
		if typ == "ini" {
			typ = "toml"
		}

		v.SetConfigType(typ)
		v.OnConfigChange(func(in fsnotify.Event) {
			updatedHostNode(v, mcf)
		})
		v.WatchConfig()
		if err := v.ReadInConfig(); err != nil {
			logging.GetLogger().Errorf("Unable to read %s: %s", mcf.Path, err)
			continue
		}
		updatedHostNode(v, mcf)
	}

	return nil
}
