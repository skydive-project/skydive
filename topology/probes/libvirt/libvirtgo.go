// +build libvirt,linux

/*
 * Copyright (C) 2018 Orange.
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
	"errors"
	"fmt"
	"sync"

	libvirtgo "github.com/libvirt/libvirt-go"
	"github.com/skydive-project/skydive/topology/probes"
)

type libvirtgoDomain struct {
	libvirtgo.Domain
}

func (d libvirtgoDomain) GetName() (string, error) {
	return d.Domain.GetName()
}

func (d libvirtgoDomain) GetXML() ([]byte, error) {
	xml, err := d.Domain.GetXMLDesc(0)
	return []byte(xml), err
}

func (d libvirtgoDomain) GetState() (DomainState, int, error) {
	state, i, err := d.Domain.GetState()
	return DomainState(state), i, err
}

type LibvirtgoMonitor struct {
	*libvirtgo.Connect
	ctx          probes.Context
	cidLifecycle int // libvirt callback id of monitor to unregister
	cidDevAdded  int // second monitor on devices added to domains
}

func (m *LibvirtgoMonitor) AllDomains() ([]domain, error) {
	domains, err := m.ListAllDomains(0)
	if err != nil {
		return nil, err
	}
	allDomains := make([]domain, len(domains))
	for i, domain := range domains {
		allDomains[i] = &libvirtgoDomain{domain}
	}
	return allDomains, nil
}

func (m *LibvirtgoMonitor) Stop() {
	if m.cidLifecycle != -1 {
		if err := m.DomainEventDeregister(m.cidLifecycle); err != nil {
			m.ctx.Logger.Errorf("Problem during deregistration: %s", err)
		}
		if err := m.DomainEventDeregister(m.cidDevAdded); err != nil {
			m.ctx.Logger.Errorf("Problem during deregistration: %s", err)
		}
	}

	if _, err := m.Close(); err != nil {
		m.ctx.Logger.Errorf("Problem during close: %s", err)
	}
}

func newMonitor(ctx context.Context, probe *Probe, wg *sync.WaitGroup) (*LibvirtgoMonitor, error) {
	if probe.uri == "" {
		probe.uri = "qemu:///system"
	}

	// The event loop must be registered WITH its poll loop active BEFORE the
	// connection is opened. Otherwise it just does not work.
	if err := libvirtgo.EventRegisterDefaultImpl(); err != nil {
		return nil, fmt.Errorf("libvirt event handler:  %s", err)
	}

	go func() {
		for ctx.Err() == nil {
			if err := libvirtgo.EventRunDefaultImpl(); err != nil {
				probe.Ctx.Logger.Errorf("libvirt poll loop problem: %s", err)
			}
		}
	}()
	conn, err := libvirtgo.NewConnectReadOnly(probe.uri)
	if err != nil {
		return nil, fmt.Errorf("Failed to create libvirt connect: %s", err)
	}

	callback := func(
		c *libvirtgo.Connect, d *libvirtgo.Domain,
		event *libvirtgo.DomainEventLifecycle,
	) {
		switch event.Event {
		case libvirtgo.DOMAIN_EVENT_UNDEFINED:
			probe.deleteDomain(libvirtgoDomain{*d})
		case libvirtgo.DOMAIN_EVENT_STARTED:
			domainNode := probe.createOrUpdateDomain(libvirtgoDomain{*d})
			interfaces, hostdevs := probe.getDomainInterfaces(libvirtgoDomain{*d}, domainNode, "")
			probe.registerInterfaces(interfaces, hostdevs)
		case libvirtgo.DOMAIN_EVENT_DEFINED, libvirtgo.DOMAIN_EVENT_SUSPENDED,
			libvirtgo.DOMAIN_EVENT_RESUMED, libvirtgo.DOMAIN_EVENT_STOPPED,
			libvirtgo.DOMAIN_EVENT_SHUTDOWN, libvirtgo.DOMAIN_EVENT_PMSUSPENDED,
			libvirtgo.DOMAIN_EVENT_CRASHED:
			probe.createOrUpdateDomain(libvirtgoDomain{*d})
		}
	}

	monitor := &LibvirtgoMonitor{Connect: conn, cidLifecycle: -1, ctx: probe.Ctx}
	if monitor.cidLifecycle, err = conn.DomainEventLifecycleRegister(nil, callback); err != nil {
		return nil, fmt.Errorf("Could not register the lifecycle event handler %s", err)
	}

	callbackDeviceAdded := func(
		c *libvirtgo.Connect, d *libvirtgo.Domain,
		event *libvirtgo.DomainEventDeviceAdded,
	) {
		domainNode := probe.getDomain(libvirtgoDomain{*d})
		interfaces, hostdevs := probe.getDomainInterfaces(libvirtgoDomain{*d}, domainNode, event.DevAlias)
		probe.registerInterfaces(interfaces, hostdevs) // 0 or 1 device changed.
	}

	if monitor.cidDevAdded, err = conn.DomainEventDeviceAddedRegister(nil, callbackDeviceAdded); err != nil {
		return nil, fmt.Errorf("Could not register the device added event handler %s", err)
	}

	wg.Add(2)

	disconnected := make(chan error, 1)
	conn.RegisterCloseCallback(func(conn *libvirtgo.Connect, reason libvirtgo.ConnectCloseReason) {
		defer wg.Done()

		monitor.Stop()
		disconnected <- errors.New("disconnected from libvirt")
	})

	go func() {
		defer wg.Done()
		defer probe.tunProcessor.Stop()

		select {
		case <-ctx.Done():
			monitor.Stop()
			break
		case <-disconnected:
		}
	}()

	return monitor, nil
}
