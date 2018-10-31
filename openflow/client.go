/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package openflow

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/goloxi"
	"github.com/skydive-project/goloxi/of10"
	"github.com/skydive-project/skydive/logging"
)

const (
	echoDuration = 3
)

var (
	// ErrContextDone is returned what the context was done or canceled
	ErrContextDone = errors.New("Context was terminated")
	// ErrConnectionTimeout is returned when a timeout was reached when trying to connect
	ErrConnectionTimeout = errors.New("Timeout while connecting")
	// ErrReaderChannelClosed is returned when the read channel was closed
	ErrReaderChannelClosed = errors.New("Reader channel was closed")
)

// Client describes an OpenFlow client
type Client struct {
	sync.RWMutex
	Conn      net.Conn
	addr      string
	reader    *bufio.Reader
	ctx       context.Context
	msgChan   chan (goloxi.Message)
	listeners []Listener
	xid       uint32
	protocol  Protocol
}

// Listener defines the interface implemented by monitor listeners
type Listener interface {
	OnMessage(goloxi.Message)
}

func (c *Client) connect(addr string) (net.Conn, error) {
	split := strings.SplitN(addr, ":", 2)
	if len(split) < 2 {
		return nil, fmt.Errorf("Invalid connection scheme: '%s'", addr)
	}
	scheme, addr := split[0], split[1]

	switch scheme {
	case "tcp":
		return net.Dial(scheme, addr)
	case "unix":
		raddr, err := net.ResolveUnixAddr("unix", addr)
		if err != nil {
			return nil, err
		}
		return net.DialUnix("unix", nil, raddr)
	default:
		return nil, fmt.Errorf("Unsupported connection scheme '%s'", scheme)
	}
}

func (c *Client) receiveMessage() (goloxi.Message, error) {
	select {
	case <-c.ctx.Done():
		return nil, ErrContextDone
	case <-time.After(30 * time.Second):
		return nil, ErrConnectionTimeout
	case msg, ok := <-c.msgChan:
		if !ok {
			return nil, ErrReaderChannelClosed
		}
		return msg, nil
	}
}

func (c *Client) handshake() (goloxi.Message, error) {
	c.SendHello()

	msg, err := c.receiveMessage()
	if err != nil {
		return nil, err
	}

	if msg.MessageType() != of10.OFPTHello {
		return nil, fmt.Errorf("Expected a first message of type Hello")
	}

	return msg, nil
}

func (c *Client) handleLoop() error {
	for {
		msg, err := c.receiveMessage()
		if err != nil {
			logging.GetLogger().Errorf("Error while receiving message: %s", err)
			return err
		}

		c.dispatchMessage(msg)

		if msg.MessageType() == of10.OFPTEchoRequest {
			c.SendMessage(c.protocol.NewEchoReply())
		}
	}
}

func (c *Client) dispatchMessage(msg goloxi.Message) {
	c.RLock()
	for _, listener := range c.listeners {
		listener.OnMessage(msg)
	}
	c.RUnlock()
}

func (c *Client) readLoop() {
	type Timeout interface {
		Timeout() bool
	}

	echoTicker := time.NewTicker(time.Second * echoDuration)
	defer echoTicker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-echoTicker.C:
			c.SendEcho()
		default:
			data, err := c.reader.Peek(8)
			if err != nil {
				if _, ok := err.(Timeout); !ok {
					logging.GetLogger().Errorf("Failed to read packet: %s", err)
					return
				}
				continue
			}

			header := of10.Header{}
			if err := header.Decode(goloxi.NewDecoder(data)); err != nil {
				logging.GetLogger().Debugf("Ignoring non OpenFlow packet: %s", err)
				continue
			}

			data = make([]byte, header.Length)
			n, err := c.reader.Read(data)
			if n != int(header.Length) {
				logging.GetLogger().Errorf("Failed to read full OpenFlow message: %s", err)
				continue
			}

			msg, err := c.protocol.DecodeMessage(data)
			if err != nil {
				logging.GetLogger().Warningf("Failed to decode message with %s: %s", err, c.protocol)
				continue
			}

			c.msgChan <- msg
		}
	}
}

type barrier struct {
	c chan goloxi.Message
}

// OnMessage is called when an OpenFlow message is received
func (b *barrier) OnMessage(msg goloxi.Message) {
	if msg.MessageName() == "OFPTBarrierReply" {
		b.c <- msg
	}
}

// PrepareMessage set the message xid and increment it
func (c *Client) PrepareMessage(msg goloxi.Message) {
	msg.SetXid(atomic.AddUint32(&c.xid, 1))
}

// SendMessage sends a message to the switch
func (c *Client) SendMessage(msg goloxi.Message) error {
	if msg.GetXid() == 0 {
		c.PrepareMessage(msg)
	}

	isBarrier := msg.MessageName() == "OFPTBarrierRequest"
	encoder := goloxi.NewEncoder()

	if err := msg.Serialize(encoder); err != nil {
		return err
	}

	if isBarrier {
		b := &barrier{c: make(chan goloxi.Message, 1)}
		c.RegisterListener(b)

		_, err := c.Conn.Write(encoder.Bytes())
		if err == nil {
			<-b.c
		}
		return nil
	}

	_, err := c.Conn.Write(encoder.Bytes())
	return err
}

// SendEcho sends an OpenFlow echo message
func (c *Client) SendEcho() error {
	return c.SendMessage(c.protocol.NewEchoRequest())
}

// SendHello sends an OpenFlow hello message
func (c *Client) SendHello() error {
	return c.SendMessage(c.protocol.NewHello())
}

// RegisterListener registers a new listener of the received messages
func (c *Client) RegisterListener(listener Listener) {
	c.Lock()
	defer c.Unlock()

	c.listeners = append(c.listeners, listener)
}

// Start monitoring the OpenFlow bridge
func (c *Client) Start(ctx context.Context) (err error) {
	c.Conn, err = c.connect(c.addr)
	if err != nil {
		return err
	}

	c.reader = bufio.NewReader(c.Conn)
	c.ctx = ctx

	go c.readLoop()

	_, err = c.handshake()
	if err != nil {
		return err
	}

	go c.handleLoop()

	logging.GetLogger().Infof("Successfully connected to OpenFlow switch")

	return nil
}

// Stop the client
func (c *Client) Stop() error {
	return nil
}

// NewClient returns a new OpenFlow client using either a UNIX socket or a TCP socket
func NewClient(addr string, protocol Protocol) (*Client, error) {
	return &Client{
		addr:     addr,
		msgChan:  make(chan goloxi.Message, 500),
		protocol: protocol,
	}, nil
}
