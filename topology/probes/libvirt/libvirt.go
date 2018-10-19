// +build libvirt

/*
 * This topology probe read list of virtual machines using libvirt
 *bindings. Make sure that version of libvirt.so matches the system
 *you want to use the skydive.
 */

package libvirt

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libvirt/libvirt-go"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"
	"github.com/skydive-project/skydive/topology/graph/realtime"
	"github.com/skydive-project/skydive/topology/probes/libvirt/helpers"
)

const MANAGER = "libvirt"

var VM_STATE_STRINGIFIED map[uint]string = map[uint]string{
	uint(libvirt.DOMAIN_NOSTATE):     "Undefined",
	uint(libvirt.DOMAIN_RUNNING):     "UP",
	uint(libvirt.DOMAIN_BLOCKED):     "BLOCKED",
	uint(libvirt.DOMAIN_PAUSED):      "PAUSED",
	uint(libvirt.DOMAIN_SHUTDOWN):    "DOWN",
	uint(libvirt.DOMAIN_CRASHED):     "CRASHED",
	uint(libvirt.DOMAIN_PMSUSPENDED): "PMSUSPENDED",
	uint(libvirt.DOMAIN_SHUTOFF):     "DOWN",
}

// LibVirtProbe describes a LibVirt topology graph that enhance the graph
type LibVirtProbe struct {
	sync.RWMutex
	Graph                     *graph.Graph
	Root                      *graph.Node
	url                       string
	Cancel                    context.CancelFunc
	State                     int64
	Connected                 atomic.Value
	Wg                        sync.WaitGroup
	GraphRealtimeHandler      *realtime.GraphHandler
	Client                    *libvirt.Connect
	libvirtCallbackId         int
	libvirtDevAddedCallbackId int
}

const STOPPED_LIBVIRT_COMMUNICATION = -1

// stringify virtual machine state (uint)
func (probe *LibVirtProbe) getVirtualMachineStatus(Vm *libvirt.Domain) string {
	vm_state, _, _ := Vm.GetState()
	state, _ := VM_STATE_STRINGIFIED[uint(vm_state)]
	return state
}

func (probe *LibVirtProbe) unregisterVM(Vm *libvirt.Domain) {
	VmName, _ := Vm.GetName()
	logging.GetLogger().Debugf("UnRegister virtual machine: %s", VmName)
	probe.GraphRealtimeHandler.RemoveNode(VmName)
}

func (probe *LibVirtProbe) changeStatusOfVm(Vm *libvirt.Domain) {
	VmName, _ := Vm.GetName()
	vmNode := probe.GraphRealtimeHandler.GetNodeByIdentifier(VmName)
	if vmNode == nil {
		logging.GetLogger().Errorf("Vm node doesn't exist in topology in topology: %s", VmName)
		return
	}
	state_stringified := probe.getVirtualMachineStatus(Vm)
	logging.GetLogger().Debugf("Vm %s status changed to: %s", VmName, state_stringified)
	probe.Graph.AddMetadata(vmNode, "State", state_stringified)
}

func (probe *LibVirtProbe) registerVM(Vm *libvirt.Domain) error {
	VmName, _ := Vm.GetName()
	XMLDesc, _ := Vm.GetXMLDesc(0)
	nodes, err := helpers.FindInterfacesVMConnectedThrough(XMLDesc, "")
	if err != nil {
		return err
	}
	logging.GetLogger().Debugf("Register virtual machine: %s", VmName)

	VmID, _ := Vm.GetID()

	logging.GetLogger().Debugf("Register VM: %s", VmName)
	state_stringified := probe.getVirtualMachineStatus(Vm)
	metadata := &graph.Metadata{
		"Type":    MANAGER,
		"Manager": MANAGER,
		"Name":    VmName,
		"LibVirt": map[string]interface{}{
			"ID":   VmID,
			"Name": VmName,
		},
		"State": state_stringified,
	}
	probe.GraphRealtimeHandler.AddNode(VmName, metadata, nodes)
	logging.GetLogger().Debugf("Finalized registration of virtual machine: %s", VmName)
	return nil
}

func (probe *LibVirtProbe) handleLibVirtEvent(Connection *libvirt.Connect, Domain *libvirt.Domain, e *libvirt.DomainEventLifecycle) {
	VmName, _ := Domain.GetName()
	logging.GetLogger().Debugf("got libvirt event: %d - %s", e.Event, VmName)
	switch e.Event {
	case libvirt.DOMAIN_EVENT_UNDEFINED:
		probe.unregisterVM(Domain)
		break
	case libvirt.DOMAIN_EVENT_STARTED:
		probe.registerVM(Domain)
		probe.changeStatusOfVm(Domain)
		break
	case libvirt.DOMAIN_EVENT_STOPPED:
		probe.changeStatusOfVm(Domain)
		break
	case libvirt.DOMAIN_EVENT_SHUTDOWN:
		probe.changeStatusOfVm(Domain)
		break
	case libvirt.DOMAIN_EVENT_DEFINED:
		probe.registerVM(Domain)
		break
	}
	return
}

func (probe *LibVirtProbe) isLibvirtCallbackConnected() bool {
	return probe.libvirtCallbackId != -1
}

func (probe *LibVirtProbe) periodicLibvirtChecker(ctx *context.Context) {
	for ctx.Err() == nil {
		if err := libvirt.EventRunDefaultImpl(); err != nil {
			logging.GetLogger().Errorf("libvirt poll loop problem: %s", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func (probe *LibVirtProbe) callbackDeviceAdded(
	c *libvirt.Connect, Vm *libvirt.Domain,
	event *libvirt.DomainEventDeviceAdded,
) {
	VmName, _ := Vm.GetName()
	XMLDesc, _ := Vm.GetXMLDesc(0)
	nodes, err := helpers.FindInterfacesVMConnectedThrough(XMLDesc, event.DevAlias)
	if err != nil {
		return
	}
	probe.GraphRealtimeHandler.UpdateConnectionsForNode(VmName, nodes)
}

func (probe *LibVirtProbe) Connect() error {
	var err error

	logging.GetLogger().Debugf("Connecting to virsh daemon: %s", probe.url)
	// explanation why we need this stuff is here:
	// https://libvirt.org/html/libvirt-libvirt-event.html#virEventRegisterImpl
	// and here https://libvirt.org/html/libvirt-libvirt-event.html#virEventRunDefaultImpl
	libvirt.EventRegisterDefaultImpl()
	probe.Client, err = libvirt.NewConnectReadOnly(probe.url)
	if err != nil {
		logging.GetLogger().Errorf("Failed to create client to libvirt daemon: %s", err.Error())
		return err
	}
	// register libvirt callback
	libvirtCallbackId, err := probe.Client.DomainEventLifecycleRegister(nil, probe.handleLibVirtEvent)
	if err != nil {
		logging.GetLogger().Errorf("Not able to connect to libvirt event loop. Error: %s", err.Error())
		return err
	} else {
		probe.libvirtCallbackId = libvirtCallbackId
		logging.GetLogger().Infof("Subscribed to libvirt events. Callback id: %d", libvirtCallbackId)
	}
	probe.libvirtDevAddedCallbackId, err = probe.Client.DomainEventDeviceAddedRegister(nil, probe.callbackDeviceAdded)
	if err != nil {
		logging.GetLogger().Errorf(
			"Could not register the device added event handler %s", err)
		return err
	}
	ctx, cancel := context.WithCancel(context.Background())

	probe.Cancel = cancel
	probe.Wg.Add(1)

	probe.Connected.Store(true)
	defer probe.Connected.Store(false)

	go func() {
		defer probe.Wg.Done()
		defer probe.periodicLibvirtChecker(ctx)
		doms, err := probe.Client.ListAllDomains(0)
		if err != nil {
			logging.GetLogger().Errorf("Failed to get virtual machines: %s", err.Error())
			return
		}
		for _, Vm := range doms {
			err = probe.registerVM(&Vm)
		}
	}()
	return nil
}

// Start the probe
func (probe *LibVirtProbe) Start() {
	if !atomic.CompareAndSwapInt64(&probe.State, common.StoppedState, common.RunningState) {
		return
	}
	go func() {
		probe.Connect()
		probe.Wg.Wait()
	}()
}

// Stop the probe
func (probe *LibVirtProbe) Stop() {
	if !atomic.CompareAndSwapInt64(&probe.State, common.RunningState, common.StoppingState) {
		return
	}
	defer probe.Client.Close()

	if probe.Connected.Load() == true {
		probe.Cancel()
		probe.Wg.Wait()
	}
	// if libvirt event callback connected, deregister it and so stop
	// all our libvirt related processes
	if probe.isLibvirtCallbackConnected() {
		probe.libvirtCallbackId = STOPPED_LIBVIRT_COMMUNICATION
		probe.Client.DomainEventDeregister(probe.libvirtCallbackId)
		probe.Client.DomainEventDeregister(probe.libvirtDevAddedCallbackId)
	}
	probe.Graph.RemoveEventListener(probe.GraphRealtimeHandler)
	atomic.StoreInt64(&probe.State, common.StoppedState)
}

// NewDProbe creates a new topology libvirt probe
func NewProbe(g *graph.Graph, root *graph.Node) (*LibVirtProbe, error) {
	probe := &LibVirtProbe{
		Graph:                g,
		Root:                 root,
		url:                  config.GetString("libvirt.url"),
		GraphRealtimeHandler: realtime.MakeGraphHandler(g, "Name"),
		State:                common.StoppedState,
		libvirtCallbackId:    STOPPED_LIBVIRT_COMMUNICATION,
	}
	g.AddEventListener(probe.GraphRealtimeHandler)
	return probe, nil
}
