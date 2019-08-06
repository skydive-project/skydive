// +build collectd

/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
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

package main

/*
#cgo CFLAGS:
#cgo LDFLAGS: -Wl,-unresolved-symbols=ignore-all

#include <stdio.h>

#include "plugin.h"

static char *config_get_string_value(const oconfig_item_t *ci) {
	if ((ci->values_num != 1) || (ci->values[0].type != OCONFIG_TYPE_STRING)) {
		return NULL;
	}

	return ci->values[0].value.string;
}

extern int SkydivePluginConfig(oconfig_item_t *ci);
static int skydive_plugin_config(oconfig_item_t *ci) {
    return SkydivePluginConfig(ci);
}

extern int SkydivePluginInit();
static int skydive_plugin_init() {
    return SkydivePluginInit();
}

extern int SkydivePluginShutdown();
static int skydive_plugin_shutdown() {
    return SkydivePluginShutdown();
}

static void register_complex_config() {
	plugin_register_complex_config("skydive", skydive_plugin_config);
}

static void register_init() {
	plugin_register_init("skydive", skydive_plugin_init);
}

static void register_shutdown() {
	plugin_register_shutdown("skydive", skydive_plugin_shutdown);
}

extern int SkydiveSend(data_set_t *, value_list_t *, user_data_t *);
static int skydive_send(const data_set_t *ds, const value_list_t *vl, user_data_t *ud) {
	return SkydiveSend((data_set_t *)ds, (value_list_t *)vl, ud);
}

static void register_write() {
	plugin_register_write("skydive", skydive_send, NULL);
}

static gauge_t get_gauge(const value_list_t *vl, int i) {
	return vl->values[i].gauge;
}

static gauge_t get_counter(const value_list_t *vl, int i) {
	return vl->values[i].counter;
}

static gauge_t get_derive(const value_list_t *vl, int i) {
	return vl->values[i].derive;
}

static gauge_t get_absolute(const value_list_t *vl, int i) {
	return vl->values[i].absolute;
}
*/
import "C"
import (
	"net/http"
	"net/url"
	"unsafe"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	gws "github.com/skydive-project/skydive/graffiti/websocket"
	shttp "github.com/skydive-project/skydive/http"
	ws "github.com/skydive-project/skydive/websocket"
)

const (
	filter = "G.V().Has('Type', 'host').SubGraph()"
)

var (
	subscriber *ws.StructSpeaker
	publisher  *ws.StructSpeaker

	rootNodeID graph.Identifier
)

type listener struct {
	ws.DefaultSpeakerEventHandler
}

// Value Collectd data value
type Value struct {
	Type  string
	Value int64
}

// Entry Collectd data entry
type Entry struct {
	Plugin         string
	PluginInstance string
	Type           string
	TypeInstance   string
	Values         []Value
}

// SkydiveSend send Collectd entry to Skydive
//export SkydiveSend
func SkydiveSend(ds *C.data_set_t, vl *C.value_list_t, ud *C.user_data_t) C.int {
	entry := Entry{
		Plugin:         C.GoString(&vl.plugin[0]),
		PluginInstance: C.GoString(&vl.plugin_instance[0]),
		Type:           C.GoString(&vl._type[0]),
		TypeInstance:   C.GoString(&vl.type_instance[0]),
	}

	sourceSize := unsafe.Sizeof(C.data_source_t{})
	for i := 0; i < int(ds.ds_num); i++ {
		source := (*C.data_source_t)(unsafe.Pointer(uintptr(unsafe.Pointer(ds.ds)) + sourceSize*uintptr(i)))

		if source._type == C.DS_TYPE_GAUGE {
			entry.Values = append(entry.Values,
				Value{
					Type:  "gauge",
					Value: int64(C.get_gauge(vl, C.int(i))),
				})
		} else if source._type == C.DS_TYPE_COUNTER {
			entry.Values = append(entry.Values,
				Value{
					Type:  "counter",
					Value: int64(C.get_counter(vl, C.int(i))),
				})
		} else if source._type == C.DS_TYPE_DERIVE {
			entry.Values = append(entry.Values,
				Value{
					Type:  "derive",
					Value: int64(C.get_derive(vl, C.int(i))),
				})
		} else if source._type == C.DS_TYPE_ABSOLUTE {
			entry.Values = append(entry.Values,
				Value{
					Type:  "absolute",
					Value: int64(C.get_absolute(vl, C.int(i))),
				})
		}
	}

	if len(rootNodeID) == 0 {
		return 0
	}

	key := "Collectd." + entry.Plugin
	if entry.PluginInstance != "" {
		key += "." + entry.PluginInstance
	}
	if entry.Type != entry.Plugin {
		key += "." + entry.Type
	}

	if entry.TypeInstance == "" {
		key += ".value"
	} else {
		key += "." + entry.TypeInstance
	}

	msg := gws.PartiallyUpdatedMsg{
		ID: rootNodeID,
		Ops: []graph.PartiallyUpdatedOp{
			{
				Type:  graph.PartiallyUpdatedAddOpType,
				Key:   key,
				Value: entry.Values,
			},
		},
	}

	Logger.Debugf("send: %+v", msg)
	publisher.SendMessage(gws.NewStructMessage(gws.NodePartiallyUpdatedMsgType, msg))

	return 0
}

// OnConnected websocket listener
func (s *listener) OnConnected(c ws.Speaker) {
	Logger.Infof("connected to %s", c.GetHost())
	subscriber.SendMessage(gws.NewStructMessage(gws.SyncRequestMsgType, gws.SyncRequestMsg{}))
}

// OnStructMessage websocket listenet
func (s *listener) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	if msg.Status != http.StatusOK {
		Logger.Errorf("request error: %v", msg)
		return
	}

	msgType, obj, err := gws.UnmarshalMessage(msg)
	if err != nil {
		Logger.Error("unable to parse websocket message: %s", err)
		return
	}

	switch msgType {
	case gws.SyncReplyMsgType:
		r := obj.(*gws.SyncMsg)

		for _, n := range r.Nodes {
			rootNodeID = n.ID
		}
	case gws.NodeAddedMsgType, gws.NodeUpdatedMsgType:
		rootNodeID = (obj.(*graph.Node)).ID
	}

	if len(rootNodeID) > 0 {
		Logger.Debugf("set root id: %s", rootNodeID)
	}
}

// SkydivePluginConfig configures the Skydive Collectd plugin
//export SkydivePluginConfig
func SkydivePluginConfig(ci *C.oconfig_item_t) C.int {
	var address string
	var username string
	var password string

	size := unsafe.Sizeof(C.oconfig_item_t{})
	for i := 0; i < int(ci.children_num); i++ {
		child := (*C.oconfig_item_t)(unsafe.Pointer(uintptr(unsafe.Pointer(ci.children)) + size*uintptr(i)))

		switch C.GoString(child.key) {
		case "Address":
			s := C.config_get_string_value(child)
			if unsafe.Pointer(s) == nil {
				Logger.Error("unable to read the Address, please check the configuration file")
				return C.int(-1)
			}
			address = C.GoString(s)
		case "Username":
			s := C.config_get_string_value(child)
			if unsafe.Pointer(s) == nil {
				Logger.Error("unable to read the Username, please check the configuration file")
				return C.int(-1)
			}
			username = C.GoString(s)
		case "Password":
			s := C.config_get_string_value(child)
			if unsafe.Pointer(s) == nil {
				Logger.Error("unable to read the Password, please check the configuration file")
				return C.int(-1)
			}
			password = C.GoString(s)
		}
	}

	authOpts := &shttp.AuthenticationOpts{
		Username: username,
		Password: password,
	}

	headers := http.Header{
		"X-Websocket-Namespace": {gws.Namespace},
	}

	if len(address) == 0 {
		address = "127.0.0.1:8081"
	}

	u, err := url.Parse("ws://" + address + "/ws/publisher")
	if err != nil {
		Logger.Infof("unable to parse the Address: %s, please check the configuration file", address)
		return C.int(-1)
	}

	client, err := config.NewWSClient("collectd", u, ws.ClientOpts{AuthOpts: authOpts, Headers: headers, Logger: &Logger})
	if err != nil {
		Logger.Infof("unable to initialize the websocket client: %s", err)
		return C.int(-1)
	}

	publisher = client.UpgradeToStructSpeaker()

	// NOTE(safchain) for now only subscribe to the host node
	headers = http.Header{
		"X-Gremlin-Filter": {filter},
	}

	if u, err = url.Parse("ws://" + address + "/ws/subscriber"); err != nil {
		Logger.Infof("unable to parse the Address: %s, please check the configuration file", address)
		return C.int(-1)
	}

	client, err = config.NewWSClient("collectd", u, ws.ClientOpts{AuthOpts: authOpts, Headers: headers, Logger: &Logger})
	if err != nil {
		Logger.Infof("unable to initialize the websocket client: %s", err)
		return C.int(-1)
	}

	l := &listener{}
	client.AddEventHandler(l)

	subscriber = client.UpgradeToStructSpeaker()
	subscriber.AddStructMessageHandler(l, []string{gws.Namespace})

	publisher.Start()
	subscriber.Start()

	C.register_write()

	return C.int(0)
}

// SkydivePluginInit initializes the Skydive Collectd plugin
//export SkydivePluginInit
func SkydivePluginInit() C.int {
	return C.int(0)
}

// SkydivePluginShutdown stop all the listeners
//export SkydivePluginShutdown
func SkydivePluginShutdown() C.int {
	publisher.Stop()
	subscriber.Stop()

	return C.int(0)
}

//export module_register
func module_register() {
	C.register_complex_config()
	C.register_init()
	C.register_shutdown()

}

func main() {}
