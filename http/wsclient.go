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

package http

import (
	"encoding/json"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

type WSMessageClientMessageHandler interface {
	OnWSMessage(c *WSAsyncClient, m WSMessage)
}

type WSClientNamespaceEventHandler struct {
	eventHandlersLock sync.RWMutex
	nsEventHandlers   map[string][]WSMessageClientMessageHandler
}

func (a *WSClientNamespaceEventHandler) AddMessageHandler(h WSMessageClientMessageHandler, namespaces []string) {
	a.eventHandlersLock.Lock()
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := a.nsEventHandlers[ns]; !ok {
			a.nsEventHandlers[ns] = []WSMessageClientMessageHandler{h}
		} else {
			a.nsEventHandlers[ns] = append(a.nsEventHandlers[ns], h)
		}
	}
	a.eventHandlersLock.Unlock()
}

func (a *WSClientNamespaceEventHandler) OnMessage(c *WSAsyncClient, m []byte) {
	var msg WSMessage
	if err := json.Unmarshal(m, &msg); err != nil {
		logging.GetLogger().Errorf("Error while decoding WSMessage %s", err.Error())
		return
	}

	for _, l := range a.nsEventHandlers[msg.Namespace] {
		l.OnWSMessage(c, msg)
	}
	for _, l := range a.nsEventHandlers[WilcardNamespace] {
		l.OnWSMessage(c, msg)
	}
}

func NewWSClientNamespaceEventHandler() *WSClientNamespaceEventHandler {
	return &WSClientNamespaceEventHandler{
		nsEventHandlers: make(map[string][]WSMessageClientMessageHandler),
	}
}

type WSClientEventHandler interface {
	OnMessage(c *WSAsyncClient, m []byte)
	OnConnected(c *WSAsyncClient)
	OnDisconnected(c *WSAsyncClient)
}

type DefaultWSClientEventHandler struct {
}

type WSAsyncClient struct {
	sync.RWMutex
	Host              string
	ClientType        common.ServiceType
	Addr              string
	Port              int
	Path              string
	AuthClient        *AuthenticationClient
	messages          chan string
	read              chan []byte
	quit              chan bool
	wg                sync.WaitGroup
	wsConn            *websocket.Conn
	eventHandlers     []WSClientEventHandler
	eventHandlersLock sync.RWMutex
	connected         atomic.Value
	running           atomic.Value
}

type WSMessageAsyncClientPool struct {
	sync.RWMutex
	*WSClientNamespaceEventHandler
	master            *WSMessageAsyncClient
	masterLock        sync.RWMutex
	clients           []*WSMessageAsyncClient
	eventHandlers     []WSClientEventHandler
	eventHandlersLock sync.RWMutex
}

func (d *DefaultWSClientEventHandler) OnMessage(c *WSAsyncClient, m []byte) {
}

func (d *DefaultWSClientEventHandler) OnConnected(c *WSAsyncClient) {
}

func (d *DefaultWSClientEventHandler) OnDisconnected(c *WSAsyncClient) {
}

func (c *WSAsyncClient) queueMessage(m string) {
	if !c.IsConnected() {
		return
	}

	c.messages <- m
}

func (c *WSAsyncClient) SendWSMessage(m *WSMessage) {
	c.queueMessage(m.String())
}

func (c *WSAsyncClient) IsConnected() bool {
	return c.connected.Load() == true
}

func (c *WSAsyncClient) SendMessage(msg string) error {
	w, err := c.wsConn.NextWriter(websocket.TextMessage)
	if err != nil {
		return err
	}

	_, err = io.WriteString(w, msg)
	if err != nil {
		return err
	}

	return w.Close()
}

func (c *WSAsyncClient) scheme() string {
	if config.IsTLSenabled() == true {
		return "wss://"
	}
	return "ws://"
}

func (c *WSAsyncClient) connect() {
	var err error
	host := c.Addr + ":" + strconv.FormatInt(int64(c.Port), 10)
	endpoint := c.scheme() + host + c.Path
	headers := http.Header{
		"X-Host-ID":             {c.Host},
		"Origin":                {endpoint},
		"X-Client-Type":         {c.ClientType.String()},
		"X-Websocket-Namespace": {WilcardNamespace},
	}

	if c.AuthClient != nil {
		if err = c.AuthClient.Authenticate(); err != nil {
			logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
			return
		}
		c.AuthClient.SetHeaders(headers)
	}

	d := websocket.Dialer{
		Proxy:           http.ProxyFromEnvironment,
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
	certPEM := config.GetConfig().GetString("agent.X509_cert")
	keyPEM := config.GetConfig().GetString("agent.X509_key")
	if certPEM != "" && keyPEM != "" {
		d.TLSClientConfig = common.SetupTLSClientConfig(certPEM, keyPEM)
		checkTLSConfig(d.TLSClientConfig)
	}
	c.wsConn, _, err = d.Dial(endpoint, headers)

	if err != nil {
		logging.GetLogger().Errorf("Unable to create a WebSocket connection %s : %s", endpoint, err.Error())
		return
	}
	defer c.wsConn.Close()
	c.wsConn.SetPingHandler(nil)

	c.connected.Store(true)
	defer c.connected.Store(false)

	logging.GetLogger().Infof("Connected to %s", endpoint)

	c.wg.Add(1)
	defer c.wg.Done()

	// notify connected
	c.RLock()
	for _, l := range c.eventHandlers {
		l.OnConnected(c)
	}
	c.RUnlock()

	go func() {
		for c.running.Load() == true {
			_, m, err := c.wsConn.ReadMessage()
			if err != nil {
				if c.running.Load() != false {
					c.quit <- true
				}
				break
			}

			c.read <- m
		}
	}()

	defer func() {
		c.connected.Store(false)
		c.RLock()
		for _, l := range c.eventHandlers {
			l.OnDisconnected(c)
		}
		c.RUnlock()
	}()

	for {
		select {
		case msg := <-c.messages:
			err := c.SendMessage(msg)
			if err != nil {
				logging.GetLogger().Errorf("Error while writing to the WebSocket: %s", err.Error())
			}
		case m := <-c.read:
			c.RLock()
			for _, l := range c.eventHandlers {
				l.OnMessage(c, m)
			}
			c.RUnlock()
		case <-c.quit:
			return
		}
	}
}

func (c *WSAsyncClient) Connect() {
	go func() {
		for c.running.Load() == true {
			c.connect()
			time.Sleep(1 * time.Second)
		}
	}()
}

func (c *WSAsyncClient) AddEventHandler(h WSClientEventHandler) {
	c.Lock()
	c.eventHandlers = append(c.eventHandlers, h)
	c.Unlock()
}

func (c *WSAsyncClient) Disconnect() {
	c.running.Store(false)
	if c.connected.Load() == true {
		c.quit <- true
		c.wg.Wait()
	}
}

func NewWSAsyncClient(host string, clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSAsyncClient {
	c := &WSAsyncClient{
		Host:       host,
		ClientType: clientType,
		Addr:       addr,
		Port:       port,
		Path:       path,
		AuthClient: authClient,
		messages:   make(chan string, 500),
		read:       make(chan []byte, 500),
		quit:       make(chan bool),
	}
	c.connected.Store(false)
	c.running.Store(true)
	return c
}

func NewWSAsyncClientFromConfig(clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSAsyncClient {
	host := config.GetConfig().GetString("host_id")
	return NewWSAsyncClient(host, clientType, addr, port, path, authClient)
}

type WSMessageAsyncClient struct {
	*WSAsyncClient
	DefaultWSClientEventHandler
	nsEventHandlers map[string][]WSMessageClientMessageHandler
}

func (c *WSMessageAsyncClient) AddMessageHandler(h WSMessageClientMessageHandler, namespaces []string) {
	// add this handler per namespace
	for _, ns := range namespaces {
		if _, ok := c.nsEventHandlers[ns]; !ok {
			c.nsEventHandlers[ns] = []WSMessageClientMessageHandler{h}
		} else {
			c.nsEventHandlers[ns] = append(c.nsEventHandlers[ns], h)
		}
	}
}

func (c *WSMessageAsyncClient) OnMessage(client *WSAsyncClient, m []byte) {
	var msg WSMessage
	if err := json.Unmarshal(m, &msg); err != nil {
		logging.GetLogger().Errorf("Error while decoding WSMessage %s", err.Error())
		return
	}
	c.RLock()
	for _, l := range c.nsEventHandlers[msg.Namespace] {
		l.OnWSMessage(client, msg)
	}
	for _, l := range c.nsEventHandlers[WilcardNamespace] {
		l.OnWSMessage(client, msg)
	}
	c.RUnlock()
}

func NewWSMessageAsyncClient(host string, clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSMessageAsyncClient {
	client := NewWSAsyncClient(host, clientType, addr, port, path, authClient)
	msgClient := &WSMessageAsyncClient{
		WSAsyncClient:   client,
		nsEventHandlers: make(map[string][]WSMessageClientMessageHandler),
	}
	client.AddEventHandler(msgClient)
	return msgClient
}

func NewWSMessageAsyncClientFromConfig(clientType common.ServiceType, addr string, port int, path string, authClient *AuthenticationClient) *WSMessageAsyncClient {
	host := config.GetConfig().GetString("host_id")
	return NewWSMessageAsyncClient(host, clientType, addr, port, path, authClient)
}

func (a *WSMessageAsyncClientPool) selectMaster() *WSMessageAsyncClient {
	a.RLock()
	defer a.RUnlock()

	a.masterLock.Lock()
	defer a.masterLock.Unlock()

	a.master = nil

	length := len(a.clients)
	if length == 0 {
		return nil
	}

	index := rand.Intn(length)
	for i := 0; i != length; i++ {
		if client := a.clients[index]; client != nil && client.IsConnected() {
			a.master = client
			break
		}

		if index+1 >= length {
			index = 0
		} else {
			index++
		}
	}

	return a.master
}

func (a *WSMessageAsyncClientPool) MasterClient() *WSMessageAsyncClient {
	a.masterLock.RLock()
	if m := a.master; m != nil {
		a.masterLock.RUnlock()
		return m
	}
	a.masterLock.RUnlock()

	return a.selectMaster()
}

func (a *WSMessageAsyncClientPool) BroadcastWSMessage(m *WSMessage) {
	a.RLock()
	defer a.RUnlock()

	for _, wsclient := range a.clients {
		if wsclient.IsConnected() {
			wsclient.SendWSMessage(m)
		}
	}
}

func (a *WSMessageAsyncClientPool) SendWSMessageToMaster(m *WSMessage) {
	if master := a.MasterClient(); master != nil {
		master.SendWSMessage(m)
	}
}

func (a *WSMessageAsyncClientPool) OnConnected(c *WSAsyncClient) {
	a.eventHandlersLock.RLock()
	defer a.eventHandlersLock.RUnlock()

	for _, l := range a.eventHandlers {
		l.OnConnected(c)
	}
}

func (a *WSMessageAsyncClientPool) OnDisconnected(c *WSAsyncClient) {
	// reset master
	a.masterLock.Lock()
	if a.master != nil && a.master.WSAsyncClient == c {
		a.master = nil
	}
	a.masterLock.Unlock()

	a.eventHandlersLock.RLock()
	defer a.eventHandlersLock.RUnlock()

	for _, l := range a.eventHandlers {
		l.OnDisconnected(c)
	}
}

func (a *WSMessageAsyncClientPool) OnWSMessage(c *WSAsyncClient, msg WSMessage) {
	a.eventHandlersLock.RLock()
	defer a.eventHandlersLock.RUnlock()

	for _, l := range a.nsEventHandlers[msg.Namespace] {
		l.OnWSMessage(c, msg)
	}
	for _, l := range a.nsEventHandlers[WilcardNamespace] {
		l.OnWSMessage(c, msg)
	}
}

func (a *WSMessageAsyncClientPool) AddWSMessageAsyncClient(client *WSMessageAsyncClient) {
	a.Lock()
	defer a.Unlock()

	a.clients = append(a.clients, client)
	client.AddEventHandler(a)
}

func (a *WSMessageAsyncClientPool) ConnectAll() {
	a.RLock()
	defer a.RUnlock()

	// shuffle connections to avoid election of the same client as master
	indexes := rand.Perm(len(a.clients))
	for _, i := range indexes {
		a.clients[i].Connect()
	}
}

func (a *WSMessageAsyncClientPool) DisconnectAll() {
	a.eventHandlersLock.Lock()
	a.eventHandlers = a.eventHandlers[:0]
	for k := range a.nsEventHandlers {
		delete(a.nsEventHandlers, k)
	}
	a.eventHandlersLock.Unlock()

	a.RLock()
	defer a.RUnlock()
	for _, client := range a.clients {
		client.Disconnect()
	}
}

func (a *WSMessageAsyncClientPool) AddEventHandler(h WSClientEventHandler) {
	a.eventHandlersLock.Lock()
	a.eventHandlers = append(a.eventHandlers, h)
	a.eventHandlersLock.Unlock()
}

func NewWSMessageAsyncClientPool() *WSMessageAsyncClientPool {
	return &WSMessageAsyncClientPool{
		WSClientNamespaceEventHandler: NewWSClientNamespaceEventHandler(),
		clients: make([]*WSMessageAsyncClient, 0),
	}
}
