package flow

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"

	"github.com/redhat-cip/skydive/logging"
)

type FlowTable struct {
	lock  sync.RWMutex
	table map[string]*Flow
}

type FlowTableAsyncNotificationUpdate interface {
	AsyncNotificationUpdate(every time.Duration)
}

func NewFlowTable() *FlowTable {
	return &FlowTable{table: make(map[string]*Flow)}
}

func (ft *FlowTable) String() string {
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	return fmt.Sprintf("%d flows", len(ft.table))
}

func (ft *FlowTable) Update(flows []*Flow) {
	ft.lock.Lock()
	for _, f := range flows {
		_, found := ft.table[f.UUID]
		if !found {
			ft.table[f.UUID] = f
		} else if f.UUID != ft.table[f.UUID].UUID {
			logging.GetLogger().Error("FlowTable Collision ", f.UUID, ft.table[f.UUID].UUID)
		}
	}
	ft.lock.Unlock()
}

type ExpireFunc func(f *Flow)

func (ft *FlowTable) expire(fn ExpireFunc, expire int64) {
	ft.lock.Lock()
	flowTableSzBefore := len(ft.table)
	for key, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last < expire {
			duration := time.Duration(fs.Last - fs.Start)
			logging.GetLogger().Debug("Expire flow %s Duration %v", f.UUID, duration)
			/* Advise Clients */
			fn(f)
			delete(ft.table, key)
		}
	}
	flowTableSz := len(ft.table)
	ft.lock.Unlock()
	logging.GetLogger().Debug("%v Expire Flow : removed %v new size %v", flowTableSzBefore-flowTableSz, flowTableSz)
}

func (ft *FlowTable) AsyncExpire(fn ExpireFunc, every time.Duration) {
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		now := <-ticker.C
		expire := now.Unix() - int64((every).Seconds())

		ft.expire(fn, expire)
	}
}

func (ft *FlowTable) IsExist(f *Flow) bool {
	ft.lock.RLock()
	_, found := ft.table[f.UUID]
	ft.lock.RUnlock()
	return found
}

func (ft *FlowTable) GetFlow(key string, packet *gopacket.Packet) (flow *Flow, new bool) {
	ft.lock.Lock()
	flow, found := ft.table[key]
	if found == false {
		flow = &Flow{}
		ft.table[key] = flow
	}
	ft.lock.Unlock()
	return flow, !found
}

func (ft *FlowTable) JSONFlowConversationEthernetPath() string {
	str := ""
	str += "{"
	//	{"nodes":[{"name":"Myriel","group":1}, ... ],"links":[{"source":1,"target":0,"value":1},...]}

	var strNodes, strLinks string
	strNodes += "\"nodes\":["
	strLinks += "\"links\":["
	pathMap := make(map[string]int)
	ethMap := make(map[string]int)
	ft.lock.RLock()
	for _, f := range ft.table {
		_, found := pathMap[f.LayersPath]
		if !found {
			pathMap[f.LayersPath] = len(pathMap)
		}

		ethFlow := f.GetStatistics().Endpoints[FlowEndpointType_ETHERNET.Value()]
		if _, found := ethMap[ethFlow.AB.Value]; !found {
			ethMap[ethFlow.AB.Value] = len(ethMap)
			strNodes += fmt.Sprintf("{\"name\":\"%s\",\"group\":%d},", ethFlow.AB.Value, pathMap[f.LayersPath])
		}
		if _, found := ethMap[ethFlow.BA.Value]; !found {
			ethMap[ethFlow.BA.Value] = len(ethMap)
			strNodes += fmt.Sprintf("{\"name\":\"%s\",\"group\":%d},", ethFlow.BA.Value, pathMap[f.LayersPath])
		}
		strLinks += fmt.Sprintf("{\"source\":%d,\"target\":%d,\"value\":%d},", ethMap[ethFlow.AB.Value], ethMap[ethFlow.BA.Value], ethFlow.AB.Bytes+ethFlow.BA.Bytes)
	}
	ft.lock.RUnlock()
	strNodes = strings.TrimRight(strNodes, ",")
	strNodes += "]"
	strLinks = strings.TrimRight(strLinks, ",")
	strLinks += "]"
	str += strNodes + "," + strLinks
	str += "}"
	return str
}

func (ft *FlowTable) NewFlowTableFromFlows(flows []*Flow) *FlowTable {
	nft := NewFlowTable()
	nft.Update(flows)
	return nft
}

/* Return a new FlowTable that contain <last> active flows */
func (ft *FlowTable) FilterLast(last time.Duration) []*Flow {
	var flows []*Flow
	selected := time.Now().Unix() - int64((last).Seconds())
	ft.lock.RLock()
	for _, f := range ft.table {
		fs := f.GetStatistics()
		if fs.Last >= selected {
			flows = append(flows, f)
		}
	}
	ft.lock.RUnlock()
	return flows
}
