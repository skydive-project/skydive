/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package ovsdb

import (
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/socketplane/libovsdb"

	"github.com/skydive-project/skydive/logging"
)

type OvsClient struct {
	ovsdb     *libovsdb.OvsdbClient
	connected uint64
}

type OvsMonitorHandler interface {
	OnOvsBridgeAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsBridgeDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsBridgeUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsInterfaceUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortAdd(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortDel(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
	OnOvsPortUpdate(monitor *OvsMonitor, uuid string, row *libovsdb.RowUpdate)
}

type OvsMonitor struct {
	sync.RWMutex
	Protocol        string
	Target          string
	OvsClient       *OvsClient
	MonitorHandlers []OvsMonitorHandler
	bridgeCache     map[string]string
	interfaceCache  map[string]string
	portCache       map[string]string
	columnsExcluded map[string]bool
	ticker          *time.Ticker
	done            chan struct{}
}

const ConnectionPollInterval time.Duration = 4 * time.Second

type Notifier struct {
	monitor *OvsMonitor
}

func (n Notifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
	n.monitor.updateHandler(&tableUpdates)
}

func (n Notifier) Locked([]interface{}) {
}

func (n Notifier) Stolen([]interface{}) {
}

func (n Notifier) Echo([]interface{}) {
}

func (n Notifier) Disconnected(c *libovsdb.OvsdbClient) {
	/* trigger re-connection */
	atomic.StoreUint64(&n.monitor.OvsClient.connected, 0)
}

func (o *OvsClient) Exec(operations ...libovsdb.Operation) ([]libovsdb.OperationResult, error) {
	if atomic.LoadUint64(&o.connected) == 0 {
		return nil, errors.New("OVSDB client is not connected")
	}

	result, err := o.ovsdb.Transact("Open_vSwitch", operations...)
	if err != nil {
		return nil, errors.New(
			"Replies number should be atleast equal to number of Operations ")
	}

	if len(result) < len(operations) {
		return nil, errors.New(
			"Replies number should be atleast equal to number of Operations ")
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

func (o *OvsMonitor) updateHandler(updates *libovsdb.TableUpdates) {
	for name, tableUpdate := range updates.Updates {
		switch name {
		case "Interface":
			o.interfaceUpdateHandler(&tableUpdate)
		case "Bridge":
			o.bridgeUpdateHandler(&tableUpdate)
		case "Port":
			o.portUpdateHandler(&tableUpdate)
		}
	}
}

func (o *OvsMonitor) setMonitorRequests(table string, r *map[string]libovsdb.MonitorRequest) error {
	schema, ok := o.OvsClient.ovsdb.Schema["Open_vSwitch"]
	if !ok {
		return errors.New("invalid Database Schema")
	}

	var columns []string
	for column := range schema.Tables[table].Columns {
		if _, found := o.columnsExcluded[column]; !found {
			columns = append(columns, column)
		}
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

func (o *OvsMonitor) AddMonitorHandler(handler OvsMonitorHandler) {
	o.Lock()
	defer o.Unlock()

	o.MonitorHandlers = append(o.MonitorHandlers, handler)
}

func (o *OvsMonitor) ExcludeColumn(column string) {
	o.columnsExcluded[column] = true
}

func (o *OvsMonitor) monitorOvsdb() error {
	ovsdb, err := libovsdb.ConnectUsingProtocol(o.Protocol, o.Target)
	if err != nil {
		return err
	}
	o.OvsClient.ovsdb = ovsdb
	atomic.StoreUint64(&o.OvsClient.connected, 1)

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

	updates, err := ovsdb.Monitor("Open_vSwitch", "", requests)
	if err != nil {
		return err
	}

	o.updateHandler(updates)

	return nil
}

// startMonitoring() is a watchdog of OvsClient connection with ConnectionPollInterval
func (o *OvsMonitor) startMonitoring() error {
	o.ticker = time.NewTicker(ConnectionPollInterval)

	for {
		select {
		case <-o.ticker.C:
			if o.isConnected() {
				continue
			}
			if err := o.monitorOvsdb(); err != nil {
				logging.GetLogger().Errorf(": re-connect error %v, will try again", err)
			}
		case <-o.done:
			break
		}
	}
}

func (o *OvsMonitor) StartMonitoring() {
	o.monitorOvsdb()
	go o.startMonitoring()
}

func (o *OvsMonitor) StopMonitoring() {
	if o.OvsClient != nil {
		o.done <- struct{}{}
		if o.isConnected() == true {
			o.OvsClient.ovsdb.Disconnect()
		}
	}
}

func NewOvsMonitor(protcol string, target string) *OvsMonitor {
	return &OvsMonitor{
		Protocol:        protcol,
		Target:          target,
		OvsClient:       &OvsClient{ovsdb: nil, connected: 0},
		bridgeCache:     make(map[string]string),
		interfaceCache:  make(map[string]string),
		portCache:       make(map[string]string),
		columnsExcluded: make(map[string]bool),
		ticker:          nil,
		done:            make(chan struct{}),
	}
}
