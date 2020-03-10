// +build linux

/*
 * Copyright (C) 2019 Orange.
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

package netlink

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/avast/retry-go"
	"github.com/vishvananda/netlink"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/topology"
)

type pendingVf struct {
	intf *graph.Node
	vfid int
}

// PciFromString transforms the symbolic representation of a pci address
// in an unsigned integer.
func PciFromString(businfo string) (uint32, error) {
	var domain, bus, device, function uint32
	_, err := fmt.Sscanf(
		businfo,
		"%04x:%02x:%02x.%01x",
		&domain, &bus, &device, &function)
	if err != nil {
		return 0, err
	}
	return ((domain << 16) | (bus << 8) | (device << 3) | function), nil
}

// PciToString transforms un unsigned integer representing a pci address in
// the symbolic representation as used by the Linux kernel.
func PciToString(address uint32) string {
	return fmt.Sprintf(
		"%04x:%02x:%02x.%01x",
		address>>16,
		(address>>8)&0xff,
		(address>>3)&0x1f,
		address&0x7)
}

/* readIntFile reads a file containing an integer. This function targets
specifically files from /sys or /proc */
func readIntFile(path string) (int, error) {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return -1, err
	}
	result, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		return -1, err
	}
	return result, nil
}

/* ProcessNode, the action associated to a pendingVf connects the node of actual
   virtual to the VF node associated to the physical interface */
func (pending *pendingVf) ProcessNode(g *graph.Graph, node *graph.Node) bool {
	tr := g.StartMetadataTransaction(node)
	tr.AddMetadata("VfID", int64(pending.vfid))
	if err := tr.Commit(); err != nil {
		logging.GetLogger().Errorf("Metadata transaction failed: %s", err)
	}
	if !topology.HaveLink(g, pending.intf, node, "vf") {
		if _, err := topology.AddLink(g, pending.intf, node, "vf", nil); err != nil {
			logging.GetLogger().Error(err)
		}
	}
	return false
}

// errZeroVfs is used as a catchable exception rather than an error
var errZeroVfs = errors.New("zero VFS")

/* handleSriov adds a node for each virtual function declared. It takes
   care of finding the PCI address of each VF */
func (u *Probe) handleSriov(
	graph *graph.Graph,
	intf *graph.Node,
	id int,
	businfo string,
	attrsVfs []netlink.VfInfo,
	name string,
) {
	var pciaddress uint32
	var offset, stride int
	var link netlink.Link
	var err error
	var numVfs int

	// Yes we have to wait. May be considered as a race bug in Linux kernel
	numVfsFile := fmt.Sprintf(
		"/sys/bus/pci/devices/%s/sriov_numvfs", businfo)

	if _, err = os.Stat(numVfsFile); os.IsNotExist(err) {
		// There is no VFS to monitor.
		return
	}
	err = retry.Do(
		func() error {
			numVfs, err = readIntFile(numVfsFile)
			if err != nil {
				return err
			}
			if numVfs == 0 {
				return errZeroVfs
			}
			return nil
		}, retry.Delay(10*time.Millisecond))
	if err != nil && err != errZeroVfs {
		logging.GetLogger().Errorf(
			"SR-IOV: cannot get numvfs of PCI - %s", err)
		return
	}
	err = retry.Do(
		func() error {
			link, err = netlink.LinkByIndex(id)
			if err != nil {
				return err
			}
			attrsVfs = link.Attrs().Vfs
			if len(attrsVfs) != numVfs {
				return errors.New("numVFS != #attrs.Vfs")
			}
			return nil
		}, retry.Delay(10*time.Millisecond))

	if err != nil {
		logging.GetLogger().Errorf(
			"SR-IOV: cannot get attributes of VFs - %s", err)
		return
	}

	pciaddress, err = PciFromString(businfo)
	if err != nil {
		logging.GetLogger().Errorf(
			"SR-IOV: cannot parse PCI address - %s", err)
		return
	}
	offsetFile := fmt.Sprintf(
		"/sys/bus/pci/devices/%s/sriov_offset", businfo)
	offset, err = readIntFile(offsetFile)
	if err != nil {
		logging.GetLogger().Errorf(
			"SR-IOV: cannot get offset of PCI - %s", err)
		return
	}
	strideFile := fmt.Sprintf(
		"/sys/bus/pci/devices/%s/sriov_stride", businfo)
	stride, err = readIntFile(strideFile)
	if err != nil {
		logging.GetLogger().Errorf(
			"SR-IOV: cannot get stride of PCI - %s", err)
		return
	}

	var vfs VFS
	for _, vf := range attrsVfs {
		vfs = append(vfs, &VF{
			ID:        int64(vf.ID),
			LinkState: int64(vf.LinkState),
			MAC:       vf.Mac.String(),
			Qos:       int64(vf.Qos),
			Spoofchk:  vf.Spoofchk,
			TxRate:    int64(vf.TxRate),
			Vlan:      int64(vf.Vlan),
		})
	}

	graph.Lock()
	defer graph.Unlock()

	tr := graph.StartMetadataTransaction(intf)
	tr.AddMetadata("Vfs", vfs)
	if err = tr.Commit(); err != nil {
		logging.GetLogger().Errorf("Metadata transaction failed: %s", err)
	}

	for _, vf := range attrsVfs {
		id := vf.ID
		pciVf := pciaddress + (uint32)(offset+id*stride)
		vfAddress := PciToString(pciVf)
		pending := pendingVf{
			intf: intf,
			vfid: id,
		}
		u.sriovProcessor.DoAction(&pending, vfAddress)
	}
}
