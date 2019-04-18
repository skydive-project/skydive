/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package ovsdb

import (
	"errors"
	"reflect"
	"sync/atomic"
	"time"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

// OvsClient describes an OVS database client connection
type OvsClient struct {
	common.RWMutex
	ovsdb     *libovsdb.OvsdbClient
	connected uint64
}

// OvsMonitorHandler describes an OVS Monitor interface mechanism
type OvsMonitorHandler interface {
	OnConnected(monitor *OvsMonitor)
	OnDisconnected(monitor *OvsMonitor)
	OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsBridgeUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsUpdate(monitor *OvsMonitor, row *libovsdb.RowUpdate)
}

// DefaultOvsMonitorHandler default implementation of an handler
type DefaultOvsMonitorHandler struct {
}

// OnConnected default implementation
func (d *DefaultOvsMonitorHandler) OnConnected(monitor *OvsMonitor) {
}

// OnDisconnected default implementation
func (d *DefaultOvsMonitorHandler) OnDisconnected(monitor *OvsMonitor) {
}

// OnOvsBridgeAdd default implementation
func (d *DefaultOvsMonitorHandler) OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

// OnOvsBridgeDel default implementation
func (d *DefaultOvsMonitorHandler) OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

// OnOvsBridgeUpdate default implementation
func (d *DefaultOvsMonitorHandler) OnOvsBridgeUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

//OnOvsInterfaceAdd default implementation
func (d *DefaultOvsMonitorHandler) OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

// OnOvsInterfaceDel default implementation
func (d *DefaultOvsMonitorHandler) OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

// OnOvsInterfaceUpdate default implementation
func (d *DefaultOvsMonitorHandler) OnOvsInterfaceUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

//OnOvsPortAdd default implementation
func (d *DefaultOvsMonitorHandler) OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

//OnOvsPortDel default implementation
func (d *DefaultOvsMonitorHandler) OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

//OnOvsPortUpdate default implementation
func (d *DefaultOvsMonitorHandler) OnOvsPortUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate) {
}

//OnOvsUpdate default implementation
func (d *DefaultOvsMonitorHandler) OnOvsUpdate(monitor *OvsMonitor, row *libovsdb.RowUpdate) {
}

// OvsMonitor describes an OVS client Monitor
type OvsMonitor struct {
	common.RWMutex
	Protocol        string
	Target          string
	OvsClient       *OvsClient
	MonitorHandlers []OvsMonitorHandler
	bridgeCache     map[string]string
	interfaceCache  map[string]string
	portCache       map[string]string
	columnsExcluded map[string][]string
	columnsIncluded map[string][]string
	ticker          *time.Ticker
	done            chan struct{}
}

// ConnectionPollInterval poll OVS database every 4 seconds
const ConnectionPollInterval time.Duration = 4 * time.Second

// Notifier describes a notification based on the monitor
type Notifier struct {
	monitor *OvsMonitor
}

// Update OVS notifier tables event
func (n Notifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	n.monitor.updateHandler(&tableUpdates)
}

// Locked OVS notifier event
func (n Notifier) Locked([]interface{}) {
}

// Stolen OVS notifier event
func (n Notifier) Stolen([]interface{}) {
}

// Echo OVS notifier event
func (n Notifier) Echo([]interface{}) {
}

// Disconnected OVS notifier event
func (n Notifier) Disconnected(c *libovsdb.OvsdbClient) {
	/* trigger re-connection */
	atomic.StoreUint64(&n.monitor.OvsClient.connected, 0)
	logging.GetLogger().Warning("Disconnected from OVSDB")

	n.monitor.Lock()
	for _, handler := range n.monitor.MonitorHandlers {
		handler.OnDisconnected(n.monitor)
	}
	n.monitor.Unlock()
}

// Exec execute a transaction on the OVS database
func (o *OvsClient) Exec(operations ...libovsdb.Operation) ([]libovsdb.OperationResult, error) {
	if atomic.LoadUint64(&o.connected) == 0 {
		return nil, errors.New("OVSDB client is not connected")
	}

	o.RLock()
	result, err := o.ovsdb.Transact("Open_vSwitch", operations...)
	o.RUnlock()

	if err != nil {
		return nil, err
	}

	if len(result) < len(operations) {
		return nil, errors.New(
			"Replies number should be atleast equal to number of Operations")
	}

	for i, o := range result {
		if o.Error != "" && i < len(operations) {
			return nil, errors.New(
				"Transaction Failed due to an error :" +
					o.Error + " details:" + o.Details)
		} else if o.Error != "" {
			return nil, errors.New(
				"Transaction Failed due to an error :" + o.Error)
		}
	}

	return result, nil
}

func (o *OvsMonitor) isConnected() bool {
	return atomic.LoadUint64(&o.OvsClient.connected) == 1
}

func (o *OvsMonitor) bridgeUpdated(bridgeUUID string, row *libovsdb.RowUpdate) {
	logging.GetLogger().Infof("Bridge \"%s(%s)\" updated",
		row.New.Fields["name"], bridgeUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsBridgeUpdate(o, bridgeUUID, row)
	}
}

func (o *OvsMonitor) bridgeAdded(bridgeUUID string, row *libovsdb.RowUpdate) {
	o.bridgeCache[bridgeUUID] = bridgeUUID

	logging.GetLogger().Infof("New bridge \"%s(%s)\" added",
		row.New.Fields["name"], bridgeUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsBridgeAdd(o, bridgeUUID, row)
	}
}

func (o *OvsMonitor) bridgeDeleted(bridgeUUID string, row *libovsdb.RowUpdate) {
	// for some reason ovs can trigger multiple time this event
	if _, ok := o.bridgeCache[bridgeUUID]; !ok {
		return
	}
	delete(o.bridgeCache, bridgeUUID)

	logging.GetLogger().Infof("Bridge \"%s(%s)\" got deleted",
		row.Old.Fields["name"], bridgeUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsBridgeDel(o, bridgeUUID, row)
	}
}

func (o *OvsMonitor) bridgeUpdateHandler(updates *libovsdb.TableUpdate) {
	empty := libovsdb.Row{}

	o.Lock()
	defer o.Unlock()

	for bridgeUUID, row := range updates.Rows {
		if !reflect.DeepEqual(row.New, empty) {
			if _, ok := o.bridgeCache[bridgeUUID]; ok {
				o.bridgeUpdated(bridgeUUID, &row)

			} else {
				o.bridgeAdded(bridgeUUID, &row)
			}
		} else {
			o.bridgeDeleted(bridgeUUID, &row)
		}
	}
}

func (o *OvsMonitor) interfaceUpdated(interfaceUUID string, row *libovsdb.RowUpdate) {
	logging.GetLogger().Infof("Interface \"%s(%s)\" updated",
		row.New.Fields["name"], interfaceUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsInterfaceUpdate(o, interfaceUUID, row)
	}
}

func (o *OvsMonitor) interfaceAdded(interfaceUUID string, row *libovsdb.RowUpdate) {
	o.interfaceCache[interfaceUUID] = interfaceUUID

	logging.GetLogger().Infof("New interface \"%s(%s)\" added",
		row.New.Fields["name"], interfaceUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsInterfaceAdd(o, interfaceUUID, row)
	}
}

func (o *OvsMonitor) interfaceDeleted(interfaceUUID string, row *libovsdb.RowUpdate) {
	// for some reason ovs can trigger multiple time this event
	if _, ok := o.interfaceCache[interfaceUUID]; !ok {
		return
	}
	delete(o.interfaceCache, interfaceUUID)

	logging.GetLogger().Infof("Interface \"%s(%s)\" got deleted",
		row.Old.Fields["name"], interfaceUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsInterfaceDel(o, interfaceUUID, row)
	}
}

func (o *OvsMonitor) interfaceUpdateHandler(updates *libovsdb.TableUpdate) {
	empty := libovsdb.Row{}

	o.Lock()
	defer o.Unlock()

	for interfaceUUID, row := range updates.Rows {
		if !reflect.DeepEqual(row.New, empty) {
			if _, ok := o.interfaceCache[interfaceUUID]; ok {
				o.interfaceUpdated(interfaceUUID, &row)
			} else {
				o.interfaceAdded(interfaceUUID, &row)
			}
		} else {
			o.interfaceDeleted(interfaceUUID, &row)
		}
	}
}

func (o *OvsMonitor) portUpdated(portUUID string, row *libovsdb.RowUpdate) {
	logging.GetLogger().Infof("Port \"%s(%s)\" updated",
		row.New.Fields["name"], portUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsPortUpdate(o, portUUID, row)
	}
}

func (o *OvsMonitor) portAdded(portUUID string, row *libovsdb.RowUpdate) {
	o.portCache[portUUID] = portUUID

	logging.GetLogger().Infof("New port \"%s(%s)\" added",
		row.New.Fields["name"], portUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsPortAdd(o, portUUID, row)
	}
}

func (o *OvsMonitor) portDeleted(portUUID string, row *libovsdb.RowUpdate) {
	// for some reason ovs can trigger multiple time this event
	if _, ok := o.portCache[portUUID]; !ok {
		return
	}
	delete(o.portCache, portUUID)

	logging.GetLogger().Infof("Port \"%s(%s)\" got deleted",
		row.Old.Fields["name"], portUUID)

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsPortDel(o, portUUID, row)
	}
}

func (o *OvsMonitor) portUpdateHandler(updates *libovsdb.TableUpdate) {
	empty := libovsdb.Row{}

	o.Lock()
	defer o.Unlock()

	for portUUID, row := range updates.Rows {
		if !reflect.DeepEqual(row.New, empty) {
			if _, ok := o.portCache[portUUID]; ok {
				o.portUpdated(portUUID, &row)
			} else {
				o.portAdded(portUUID, &row)
			}
		} else {
			o.portDeleted(portUUID, &row)
		}
	}
}

func (o *OvsMonitor) ovsUpdate(row *libovsdb.RowUpdate) {
	logging.GetLogger().Info("OpenvSwitch system updated")

	for _, handler := range o.MonitorHandlers {
		handler.OnOvsUpdate(o, row)
	}
}

func (o *OvsMonitor) ovsUpdateHandler(updates *libovsdb.TableUpdate) {
	for _, row := range updates.Rows {
		o.ovsUpdate(&row)
	}
}

func (o *OvsMonitor) updateHandler(updates *libovsdb.TableUpdates) {
	for name, tableUpdate := range updates.Updates {
		switch name {
		case "Interface":
			o.interfaceUpdateHandler(&tableUpdate)
		case "Bridge":
			o.bridgeUpdateHandler(&tableUpdate)
		case "Port":
			o.portUpdateHandler(&tableUpdate)
		case "Open_vSwitch":
			o.ovsUpdateHandler(&tableUpdate)
		}

	}
}

func (o *OvsMonitor) setMonitorRequests(table string, r *map[string]libovsdb.MonitorRequest) error {
	o.OvsClient.RLock()
	schema, ok := o.OvsClient.ovsdb.Schema["Open_vSwitch"]
	o.OvsClient.RUnlock()

	if !ok {
		return errors.New("invalid Database Schema")
	}

	selected := make(map[string]bool)

	// include everything by default
	for column := range schema.Tables[table].Columns {
		selected[column] = true
	}

	// exclude columns
	for _, selector := range []string{table, "*"} {
		if columns, found := o.columnsExcluded[selector]; found {
			for _, column := range columns {
				if column == "*" {
					for column = range schema.Tables[table].Columns {
						delete(selected, column)
					}
				} else {
					if _, found := schema.Tables[table].Columns[column]; found {
						delete(selected, column)
					}
				}
			}
		}
	}

	// include columns
	for _, selector := range []string{table, "*"} {
		if columns, found := o.columnsIncluded[selector]; found {
			for _, column := range columns {
				if column == "*" {
					for column = range schema.Tables[table].Columns {
						selected[column] = true
					}
				} else {
					if _, found := schema.Tables[table].Columns[column]; found {
						selected[column] = true
					}
				}
			}
		}
	}

	var columns []string
	for column := range selected {
		columns = append(columns, column)
	}

	requests := *r
	requests[table] = libovsdb.MonitorRequest{
		Columns: columns,
		Select: libovsdb.MonitorSelect{
			Initial: true,
			Insert:  true,
			Delete:  true,
			Modify:  true,
		},
	}

	return nil
}

// AddMonitorHandler subscribe a new monitor events handler
func (o *OvsMonitor) AddMonitorHandler(handler OvsMonitorHandler) {
	o.Lock()
	defer o.Unlock()

	o.MonitorHandlers = append(o.MonitorHandlers, handler)
}

// ExcludeColumn excludes the given table/column to be monitored. All columns can be
// excluded using "*" as column name.
func (o *OvsMonitor) ExcludeColumn(table, column string) {
	if _, ok := o.columnsExcluded[table]; !ok {
		o.columnsExcluded[table] = []string{column}
	} else {
		o.columnsExcluded[table] = append(o.columnsExcluded[table], column)
	}
}

// IncludeColumn includes the given column in the set of the monitored column.
// Columns are excluded and then included in that order. By default all the column
// are included.
func (o *OvsMonitor) IncludeColumn(table, column string) {
	if _, ok := o.columnsIncluded[table]; !ok {
		o.columnsIncluded[table] = []string{column}
	} else {
		o.columnsIncluded[table] = append(o.columnsIncluded[table], column)
	}
}

func (o *OvsMonitor) monitorOvsdb() error {
	ovsdb, err := libovsdb.ConnectUsingProtocol(o.Protocol, o.Target)
	if err != nil {
		return err
	}
	o.OvsClient.Lock()
	o.OvsClient.ovsdb = ovsdb
	o.OvsClient.Unlock()

	atomic.StoreUint64(&o.OvsClient.connected, 1)

	for _, handler := range o.MonitorHandlers {
		handler.OnConnected(o)
	}

	notifier := Notifier{monitor: o}
	ovsdb.Register(notifier)

	requests := make(map[string]libovsdb.MonitorRequest)
	err = o.setMonitorRequests("Bridge", &requests)
	if err != nil {
		return err
	}

	err = o.setMonitorRequests("Interface", &requests)
	if err != nil {
		return err
	}

	err = o.setMonitorRequests("Port", &requests)
	if err != nil {
		return err
	}

	err = o.setMonitorRequests("Open_vSwitch", &requests)
	if err != nil {
		return err
	}

	updates, err := ovsdb.Monitor("Open_vSwitch", "", requests)
	if err != nil {
		return err
	}

	o.updateHandler(updates)

	return nil
}

// startMonitoring() is a watchdog of OvsClient connection with ConnectionPollInterval
func (o *OvsMonitor) startMonitoring() error {
	connectedOnce := false
	o.ticker = time.NewTicker(ConnectionPollInterval)

	if err := o.monitorOvsdb(); err != nil {
		logging.GetLogger().Warningf("Could not connect to OVSDB(%s), will retry every %s", err, ConnectionPollInterval.String())
	}

	for {
		select {
		case <-o.ticker.C:
			if o.isConnected() {
				connectedOnce = true
				continue
			}
			if err := o.monitorOvsdb(); err != nil {
				if connectedOnce {
					logging.GetLogger().Errorf(": re-connect error %v, will try again", err)
				}
			}
		case <-o.done:
			break
		}
	}
}

// StartMonitoring start the OVS database monitoring
func (o *OvsMonitor) StartMonitoring() {
	o.monitorOvsdb()
	go o.startMonitoring()
}

// StopMonitoring stop the OVS database monitoring
func (o *OvsMonitor) StopMonitoring() {
	if o.OvsClient != nil {
		o.done <- struct{}{}
		if o.isConnected() == true {
			o.OvsClient.RLock()
			o.OvsClient.ovsdb.Disconnect()
			o.OvsClient.RUnlock()
		}
	}
}

// NewOvsMonitor creates a new monitoring probe agent on target
func NewOvsMonitor(protocol string, target string) *OvsMonitor {
	return &OvsMonitor{
		Protocol:        protocol,
		Target:          target,
		OvsClient:       &OvsClient{ovsdb: nil, connected: 0},
		bridgeCache:     make(map[string]string),
		interfaceCache:  make(map[string]string),
		portCache:       make(map[string]string),
		columnsExcluded: make(map[string][]string),
		columnsIncluded: make(map[string][]string),
		ticker:          nil,
		done:            make(chan struct{}),
	}
}
