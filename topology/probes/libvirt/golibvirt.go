// +build !libvirt,linux

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

package libvirt

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	golibvirt "github.com/digitalocean/go-libvirt"
)

type golibvirtDomain struct {
	*golibvirt.Libvirt
	golibvirt.Domain
}

func (d golibvirtDomain) GetName() (string, error) {
	return d.Name, nil
}

func (d golibvirtDomain) GetXML() ([]byte, error) {
	return d.XML(d.Name, 0)
}

func (d golibvirtDomain) GetState() (DomainState, int, error) {
	state, err := d.DomainState(d.Name)
	return state, 0, err
}

type golibvirtMonitor struct {
	libvirt *golibvirt.Libvirt
	conn    net.Conn
}

func (m *golibvirtMonitor) AllDomains() ([]domain, error) {
	domains, err := m.libvirt.Domains()
	if err != nil {
		return nil, err
	}
	allDomains := make([]domain, len(domains))
	for i, domain := range domains {
		allDomains[i] = &golibvirtDomain{Libvirt: m.libvirt, Domain: domain}
	}
	return allDomains, nil
}

func (m *golibvirtMonitor) Stop() {
	m.libvirt.Disconnect()
}

func newMonitor(ctx context.Context, probe *Probe, wg *sync.WaitGroup) (*golibvirtMonitor, error) {
	if probe.uri == "" {
		probe.uri = "unix:///var/run/libvirt/libvirt-sock"
	}

	split := strings.SplitN(probe.uri, "://", 2)
	if len(split) != 2 {
		return nil, fmt.Errorf("invalid address format %s", probe.uri)
	}

	conn, err := net.DialTimeout(split[0], split[1], 2*time.Second)
	if err != nil {
		return nil, err
	}

	libvirt := golibvirt.New(conn)
	monitor := &golibvirtMonitor{libvirt: libvirt}

	if err := libvirt.Connect(); err != nil {
		return nil, fmt.Errorf("failed to connect: %s", err)
	}

	events, err := libvirt.LifecycleEvents()
	if err != nil {
		return nil, fmt.Errorf("failed to watch lifecycle events: %s", err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer probe.tunProcessor.Stop()

		for {
			select {
			case <-ctx.Done():
				libvirt.Disconnect()
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				d := golibvirtDomain{Libvirt: libvirt, Domain: event.Dom}
				switch golibvirt.DomainEventType(event.Event) {
				case golibvirt.DomainEventUndefined:
					probe.deleteDomain(d)
				case golibvirt.DomainEventStarted, golibvirt.DomainEventDefined:
					domainNode := probe.createOrUpdateDomain(d)
					interfaces, hostdevs := probe.getDomainInterfaces(d, domainNode, "")
					probe.registerInterfaces(interfaces, hostdevs)
				case golibvirt.DomainEventSuspended,
					golibvirt.DomainEventResumed, golibvirt.DomainEventStopped,
					golibvirt.DomainEventShutdown, golibvirt.DomainEventPmsuspended,
					golibvirt.DomainEventCrashed:
					probe.createOrUpdateDomain(d)
				}
			}
		}
	}()

	return monitor, nil
}
