// Copyright (c) 2020 Sylvain Afchain
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hardware

import (
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes"
)

type Probe struct {
	Ctx probes.Context
}

// Start the probe
func (p *Probe) Start() error {
	p.Ctx.Graph.Lock()
	defer p.Ctx.Graph.Unlock()

	tr := p.Ctx.Graph.StartMetadataTransaction(p.Ctx.RootNode)
	defer tr.Commit()

	if instanceID, err := getInstanceID(); err != nil {
		p.Ctx.Logger.Error(err)
	} else if instanceID != "" {
		tr.AddMetadata("InstanceID", instanceID)
	}

	if isolated, err := getIsolatedCPUs(); err != nil {
		p.Ctx.Logger.Error(err)
	} else if isolated != nil {
		tr.AddMetadata("IsolatedCPU", isolated)
	}

	cpuInfo, err := cpu.Info()
	if err != nil {
		return err
	}

	var cpus CPUInfos
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

	tr.AddMetadata("CPU", cpus)

	hostInfo, err := host.Info()
	if err != nil {
		return err
	}

	if hostInfo.OS != "" {
		tr.AddMetadata("OS", hostInfo.OS)
	}
	if hostInfo.Platform != "" {
		tr.AddMetadata("Platform", hostInfo.Platform)
	}
	if hostInfo.PlatformFamily != "" {
		tr.AddMetadata("PlatformFamily", hostInfo.PlatformFamily)
	}
	if hostInfo.PlatformVersion != "" {
		tr.AddMetadata("PlatformVersion", hostInfo.PlatformVersion)
	}
	if hostInfo.KernelVersion != "" {
		tr.AddMetadata("KernelVersion", hostInfo.KernelVersion)
	}
	if hostInfo.VirtualizationSystem != "" {
		tr.AddMetadata("VirtualizationSystem", hostInfo.VirtualizationSystem)
	}
	if hostInfo.VirtualizationRole != "" {
		tr.AddMetadata("VirtualizationRole", hostInfo.VirtualizationRole)
	}

	if cmd, err := getKernelCmd(); err != nil {
		p.Ctx.Logger.Error(err)
	} else if cmd != nil {
		tr.AddMetadata("KernelCmdLine", cmd)
	}

	return nil
}

// Stop the probe
func (u *Probe) Stop() {
}

// NewProbe returns a BESS topology probe
func NewProbe(ctx probes.Context, bundle *probe.Bundle) (probe.Handler, error) {
	p := &Probe{
		Ctx: ctx,
	}

	return p, nil
}

// Register registers graph metadata decoders
func Register() {
	graph.NodeMetadataDecoders["CPU"] = MetadataDecoder
}
