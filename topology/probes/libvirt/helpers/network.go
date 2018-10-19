package helpers

import (
	"encoding/xml"
	"fmt"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph/realtime"
)

// Target describes the XML coding of the target of an interface in libvirt
// Address describes the XML coding of the pci addres of an interface in libvirt
type Address struct {
	Type     string `xml:"type,attr"`
	Domain   string `xml:"domain,attr"`
	Bus      string `xml:"bus,attr"`
	Slot     string `xml:"slot,attr"`
	Function string `xml:"function,attr"`
}

type Interface struct {
	Driver struct {
		Name string `xml:"name,attr"`
	} `xml:"driver"`
	Mac struct {
		Address string `xml:"address,attr"`
	} `xml:"mac"`
	InterfaceType string `xml:"type,attr"`
	Source        struct {
		Bridge string `xml:"bridge,attr"`
	} `xml:"source"`
	Target struct {
		Dev string `xml:"dev,attr"`
	} `xml:"target"`
	Alias struct {
		Name string `xml:"name,attr"`
	} `xml:"alias"`
	Address Address `xml:"address"`
}

type Domain struct {
	Interfaces []Interface `xml:"devices>interface"`
}

func getMetadataForVmConnection(itf *Interface) map[string]string {
	address := itf.Address
	formatted := fmt.Sprintf(
		"%s:%s.%s.%s.%s", address.Type, address.Domain, address.Bus,
		address.Slot, address.Function)
	return map[string]string{
		"Libvirt.MAC":     itf.Mac.Address,
		"Libvirt.Address": formatted,
		"Libvirt.Alias":   itf.Alias.Name,
		"PeerIntfMAC":     itf.Mac.Address,
	}
}

func FindInterfacesVMConnectedThrough(XMLDesc string, constraint string) (map[string]realtime.NodesConnectedToType, error) {
	InterfacesConnectedTo := make(map[string]realtime.NodesConnectedToType)
	d := Domain{}
	if err := xml.Unmarshal([]byte(XMLDesc), &d); err != nil {
		logging.GetLogger().Errorf("XML parsing error: %s", err)
		return nil, fmt.Errorf("XML parsing error: %s", err)
	}
	// Iterate through list
	for i := range d.Interfaces {
		interf := &d.Interfaces[i]
		if constraint != "" && constraint != interf.Alias.Name {
			continue
		}
		if interf.Target.Dev != "" {
			metadata := getMetadataForVmConnection(interf)
			logging.GetLogger().Debugf("Found a dev target %s", interf.Target.Dev)
			InterfacesConnectedTo[interf.Target.Dev] = realtime.NodesConnectedToType{
				Name:     interf.Target.Dev,
				Type:     realtime.VLAYER_CONNECTION_TYPE,
				Metadata: metadata,
			}
		}

	}
	return InterfacesConnectedTo, nil
}
