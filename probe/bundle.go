/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package probe

type Probe interface {
	Start()
	Stop()
}

type ProbeBundle struct {
	probes map[string]Probe
}

func (p *ProbeBundle) Start() {
	for _, p := range p.probes {
		p.Start()
	}
}

func (p *ProbeBundle) Stop() {
	for _, p := range p.probes {
		p.Stop()
	}
}

func (p *ProbeBundle) GetProbe(k string) Probe {
	if probe, ok := p.probes[k]; ok {
		return probe
	}
	return nil
}

func NewProbeBundle(p map[string]Probe) *ProbeBundle {
	return &ProbeBundle{
		probes: p,
	}
}
