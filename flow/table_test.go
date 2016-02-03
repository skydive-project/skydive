package flow

import (
	"math/rand"
	"net"
	"testing"
	"time"

	"encoding/json"
	j "github.com/gima/jsonv/src"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func forgeEthIPTCP(t *testing.T, seed int64) *gopacket.Packet {
	var options gopacket.SerializeOptions
	rnd := rand.New(rand.NewSource(seed))

	rawBytes := []byte{10, 20, 30}
	ethernetLayer := &layers.Ethernet{
		SrcMAC: net.HardwareAddr{0x00, 0x0F, 0xAA, 0xFA, 0xAA, byte(rnd.Intn(0x100))},
		DstMAC: net.HardwareAddr{0x00, 0x0D, 0xBD, 0xBD, byte(rnd.Intn(0x100)), 0xBD},
	}
	ipLayer := &layers.IPv4{
		SrcIP: net.IP{127, 0, 0, byte(rnd.Intn(0x100))},
		DstIP: net.IP{byte(rnd.Intn(0x100)), 8, 8, 8},
	}
	tcpLayer := &layers.TCP{
		SrcPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
		DstPort: layers.TCPPort(byte(rnd.Intn(0x10000))),
	}
	// And create the packet with the layers
	buffer := gopacket.NewSerializeBuffer()
	err := gopacket.SerializeLayers(buffer, options,
		ethernetLayer,
		ipLayer,
		tcpLayer,
		gopacket.Payload(rawBytes),
	)
	if err != nil {
		t.Fail()
	}

	gpacket := gopacket.NewPacket(buffer.Bytes(), layers.LayerTypeEthernet, gopacket.Default)
	return &gpacket

}

func generateFlows(t *testing.T, ft *FlowTable) []*Flow {
	flows := []*Flow{}
	for i := 0; i < 10; i++ {
		packet := forgeEthIPTCP(t, int64(i))
		flow, new := ft.GetFlow(string(i), packet)
		if !new {
			t.Fail()
		}
		err := flow.fillFromGoPacket(packet)
		if err != nil {
			t.Error("fillFromGoPacket : " + err.Error())
		}
		flows = append(flows, flow)
	}
	return flows
}

func checkFlowTable(t *testing.T, ft *FlowTable) {
	for uuid, f := range ft.table {
		if uuid != f.UUID {
			t.Error("FlowTable Collision ", uuid, f.UUID)
		}
	}
}

func NewFlowTableSimple(t *testing.T) *FlowTable {
	ft := NewFlowTable()
	var flows []*Flow
	flow := &Flow{}
	flow.UUID = "1234"
	flows = append(flows, flow)
	ft.Update(flows)
	checkFlowTable(t, ft)
	ft.Update(flows)
	checkFlowTable(t, ft)
	if "1 flows" != ft.String() {
		t.Error("We should got only 1 flow")
	}
	flow = &Flow{}
	flow.UUID = "4567"
	flows = append(flows, flow)
	ft.Update(flows)
	checkFlowTable(t, ft)
	ft.Update(flows)
	checkFlowTable(t, ft)
	if "2 flows" != ft.String() {
		t.Error("We should got only 2 flows")
	}
	return ft
}
func NewFlowTableComplex(t *testing.T) *FlowTable {
	ft := NewFlowTable()
	flows := generateFlows(t, ft)
	ft = ft.NewFlowTableFromFlows(flows)
	checkFlowTable(t, ft)
	return ft
}

func TestNewFlowTable(t *testing.T) {
	ft := NewFlowTable()
	if ft == nil {
		t.Error("new FlowTable return null")
	}
}
func TestFlowTable_String(t *testing.T) {
	ft := NewFlowTable()
	if "0 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
	ft = NewFlowTableSimple(t)
	if "2 flows" != ft.String() {
		t.Error("FlowTable too big")
	}
}
func TestFlowTable_Update(t *testing.T) {
	NewFlowTableSimple(t)
	NewFlowTableComplex(t)
}

type MyTestFlowCounter struct {
	NbFlow int
}

func (fo *MyTestFlowCounter) expireCallback(f *Flow) {
	fo.NbFlow++
}

func TestFlowTable_expire(t *testing.T) {
	const MaxInt64 = int64(^uint64(0) >> 1)
	ft := NewFlowTableComplex(t)

	fc := MyTestFlowCounter{}
	beforeNbFlow := fc.NbFlow
	ft.expire(fc.expireCallback, 0)
	afterNbFlow := fc.NbFlow
	if beforeNbFlow != 0 && afterNbFlow != 0 {
		t.Error("we should not expire a flow")
	}

	fc = MyTestFlowCounter{}
	beforeNbFlow = fc.NbFlow
	ft.expire(fc.expireCallback, MaxInt64)
	afterNbFlow = fc.NbFlow
	if beforeNbFlow != 0 && afterNbFlow != 10 {
		t.Error("we should expire all flows")
	}
}

func TestFlowTable_AsyncExpire(t *testing.T) {
	t.Skip()
}

func TestFlowTable_IsExist(t *testing.T) {
	ft := NewFlowTableSimple(t)
	flow := &Flow{}
	flow.UUID = "1234"
	if ft.IsExist(flow) == false {
		t.Fail()
	}
	flow.UUID = "12345"
	if ft.IsExist(flow) == true {
		t.Fail()
	}
}

func TestFlowTable_GetFlow(t *testing.T) {
	ft := NewFlowTableComplex(t)
	flows := generateFlows(t, ft)
	if len(flows) != 10 {
		t.Error("missing some flows ", len(flows))
	}
	packet := forgeEthIPTCP(t, int64(1234))
	_, new := ft.GetFlow("abcd", packet)
	if !new {
		t.Error("Collision in the FlowTable, should be new")
	}
	packet = forgeEthIPTCP(t, int64(1234))
	_, new = ft.GetFlow("abcd", packet)
	if new {
		t.Error("Collision in the FlowTable, should be an update")
	}
	packet = forgeEthIPTCP(t, int64(1234))
	_, new = ft.GetFlow("abcde", packet)
	if !new {
		t.Error("Collision in the FlowTable, should be a new flow")
	}
}

func isJSON(s string) bool {
	var js map[string]interface{}
	return json.Unmarshal([]byte(s), &js) == nil
}

func TestFlowTable_JSONFlowConversationEthernetPath(t *testing.T) {
	ft := NewFlowTableComplex(t)
	statStr := ft.JSONFlowConversationEthernetPath()
	if statStr == `{"nodes":[],"links":[]}` {
		t.Error("stat should not be empty")
	}
	if !isJSON(statStr) {
		t.Error("stat should be JSON format")
	}

	decoded := new(interface{})
	if err := json.Unmarshal([]byte(statStr), decoded); err != nil {
		t.Error("JSON parsing failed:", err)
	}

	schema := &j.Object{Properties: []j.ObjectItem{
		{"nodes", &j.Array{Each: &j.Object{Properties: []j.ObjectItem{
			{"name", &j.String{MinLen: 1}},
			{"group", &j.Number{Min: 0, Max: 20}}}}},
		},
		{"links", &j.Array{Each: &j.Object{Properties: []j.ObjectItem{
			{"source", &j.Number{Min: 0, Max: 20}},
			{"target", &j.Number{Min: 0, Max: 20}},
			{"value", &j.Number{Min: 0, Max: 9999}}}}},
		},
	}}
	if path, err := schema.Validate(decoded); err != nil {
		t.Errorf("Failed (%s). Path: %s", err, path)
	}
}

func TestFlowTable_NewFlowTableFromFlows(t *testing.T) {
	ft := NewFlowTableComplex(t)
	var flows []*Flow
	for _, f := range ft.table {
		flow := *f
		flows = append(flows, &flow)
	}
	ft2 := ft.NewFlowTableFromFlows(flows)
	if len(ft.table) != len(ft2.table) {
		t.Error("NewFlowTable(copy) are not the same size")
	}
	flows = flows[:0]
	for _, f := range ft.table {
		flows = append(flows, f)
	}
	ft3 := ft.NewFlowTableFromFlows(flows)
	if len(ft.table) != len(ft3.table) {
		t.Error("NewFlowTable(ref) are not the same size")
	}
}

func TestFlowTable_FilterLast(t *testing.T) {
	ft := NewFlowTableComplex(t)
	/* hack to put the FlowTable 1 second older */
	for _, f := range ft.table {
		fs := f.GetStatistics()
		fs.Start -= int64(1)
		fs.Last -= int64(1)
	}
	flows := ft.FilterLast(10 * time.Minute)
	if len(flows) != 10 {
		t.Error("FilterLast should return more/less flows", len(flows), 10)
	}
	flows = ft.FilterLast(0 * time.Minute)
	if len(flows) != 0 {
		t.Error("FilterLast should return less flows", len(flows), 0)
	}
}
