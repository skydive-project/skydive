/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package analyzer

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/flow/enhancers"
	"github.com/skydive-project/skydive/flow/storage"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/graph"
)

// FlowServerConn describes a flow server connection
type FlowServerConn interface {
	Accept() (net.Conn, error)
	Cleanup() error
}

// FlowServerConn describes a UDP flow server connection
type FlowServerUDPConn struct {
	udpConn *net.UDPConn
}

// FlowServerConn describes a TCP flow server connection
type FlowServerTCPConn struct {
	tlsConn   net.Conn
	tlsListen net.Listener
}

// FlowServer describes a flow server with pipeline enhancers mechanism
type FlowServer struct {
	Addr             string
	Port             int
	Storage          storage.Storage
	EnhancerPipeline *flow.EnhancerPipeline
	conn             FlowServerConn
	state            int64
	wgServer         sync.WaitGroup
	wgFlowsHandlers  sync.WaitGroup
	bulkInsert       int
	bulkDeadline     int
}

// Accept connection step
func (c *FlowServerTCPConn) Accept() (net.Conn, error) {
	acceptedTLSConn, err := c.tlsListen.Accept()
	if err != nil {
		return nil, err
	}

	tlsConn, ok := acceptedTLSConn.(*tls.Conn)
	if !ok {
		logging.GetLogger().Fatalf("This is not a TLS connection %v", c.tlsConn)
	}

	if err = tlsConn.Handshake(); err != nil {
		return nil, err
	}

	if state := tlsConn.ConnectionState(); state.HandshakeComplete == false {
		return nil, errors.New("TLS Handshake is not complete")
	}

	return tlsConn, nil
}

// Cleanup stop listening on the connection
func (c *FlowServerTCPConn) Cleanup() error {
	return c.tlsListen.Close()
}

// Accept connection step
func (c *FlowServerUDPConn) Accept() (net.Conn, error) {
	return c.udpConn, nil
}

// Cleanup stop listening on the connection
func (c *FlowServerUDPConn) Cleanup() error {
	return nil
}

func NewFlowServerTCPConn(addr string, port int) (*FlowServerTCPConn, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	if err != nil {
		return nil, err
	}

	certPEM := config.GetConfig().GetString("analyzer.X509_cert")
	keyPEM := config.GetConfig().GetString("analyzer.X509_key")
	clientCertPEM := config.GetConfig().GetString("agent.X509_cert")

	if len(certPEM) == 0 || len(keyPEM) == 0 {
		return nil, errors.New("Certificates are required to use TCP for flows")
	}

	cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
	if err != nil {
		logging.GetLogger().Fatalf("Can't read X509 key pair set in config : cert '%s' key '%s'", certPEM, keyPEM)
	}

	rootPEM, err := ioutil.ReadFile(clientCertPEM)
	if err != nil {
		logging.GetLogger().Fatalf("Failed to open root certificate '%s' : %s", clientCertPEM, err.Error())
	}

	roots := x509.NewCertPool()
	if ok := roots.AppendCertsFromPEM([]byte(rootPEM)); !ok {
		logging.GetLogger().Fatalf("Failed to parse root certificate '%s'", clientCertPEM)
	}

	cfgTLS := &tls.Config{
		ClientCAs:                roots,
		ClientAuth:               tls.RequireAndVerifyClientCert,
		Certificates:             []tls.Certificate{cert},
		MinVersion:               tls.VersionTLS12,
		PreferServerCipherSuites: true,
		CipherSuites: []uint16{
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		},
	}

	cfgTLS.BuildNameToCertificate()
	tlsListen, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", common.IPToString(tcpAddr.IP), port+1), cfgTLS)
	if err != nil {
		return nil, err
	}

	logging.GetLogger().Info("Analyzer listen agents on TLS socket")
	return &FlowServerTCPConn{
		tlsListen: tlsListen,
	}, nil
}

func NewFlowServerUDPConn(addr string, port int) (*FlowServerUDPConn, error) {
	host := addr + ":" + strconv.FormatInt(int64(port), 10)
	udpAddr, err := net.ResolveUDPAddr("udp", host)
	if err != nil {
		return nil, err
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	logging.GetLogger().Info("Analyzer listen agents on UDP socket")
	return &FlowServerUDPConn{udpConn: udpConn}, err
}

func (s *FlowServer) storeFlows(flows []*flow.Flow) {
	if s.Storage != nil && len(flows) > 0 {
		s.EnhancerPipeline.Enhance(flows)
		s.Storage.StoreFlows(flows)

		logging.GetLogger().Debugf("%d flows stored", len(flows))
	}
}

// handleFlowPacket can handle connection based on TCP or UDP
func (s *FlowServer) handleFlowPacket(conn net.Conn) {
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
	// each flow can be HeaderSize * RawPackets + flow size (~500)
	data := make([]byte, flow.MaxCaptureLength*flow.MaxRawPacketLimit+flow.DefaultProtobufFlowSize)

	var flowBuffer []*flow.Flow
	defer s.storeFlows(flowBuffer)

	dlTimer := time.NewTicker(time.Duration(s.bulkDeadline) * time.Second)
	defer dlTimer.Stop()

	for atomic.LoadInt64(&s.state) == common.RunningState {
		select {
		case <-dlTimer.C:
			s.storeFlows(flowBuffer)
			flowBuffer = flowBuffer[:0]
		default:
			n, err := conn.Read(data)
			if err != nil {
				timeout := false
				switch netErr := err.(type) {
				case net.Error:
					timeout = netErr.Timeout()
				case *net.OpError:
					timeout = netErr.Timeout()
				}

				if timeout {
					conn.SetDeadline(time.Now().Add(200 * time.Millisecond))
					continue
				}

				if atomic.LoadInt64(&s.state) != common.RunningState {
					return
				}

				logging.GetLogger().Errorf("Error while reading: %s", err.Error())
				return
			}

			f, err := flow.FromData(data[0:n])
			if err != nil {
				logging.GetLogger().Errorf("Error while parsing flow: %s", err.Error())
				continue
			}

			flowBuffer = append(flowBuffer, f)
			if len(flowBuffer) >= s.bulkInsert {
				s.storeFlows(flowBuffer)
				flowBuffer = flowBuffer[:0]
			}
		}
	}
}

// Start the flow server
func (s *FlowServer) Start() {
	var err error
	protocol := strings.ToLower(config.GetConfig().GetString("flow.protocol"))
	switch protocol {
	case "udp":
		s.conn, err = NewFlowServerUDPConn(s.Addr, s.Port)
	case "tcp":
		s.conn, err = NewFlowServerTCPConn(s.Addr, s.Port)
	default:
		err = fmt.Errorf("Invalid protocol %s", protocol)
	}

	if err != nil {
		logging.GetLogger().Errorf("Unable to start flow server: %s", err.Error())
		return
	}
	atomic.StoreInt64(&s.state, common.RunningState)

	s.wgServer.Add(1)
	go func() {
		defer s.wgServer.Done()

		for atomic.LoadInt64(&s.state) == common.RunningState {
			conn, err := s.conn.Accept()
			if atomic.LoadInt64(&s.state) != common.RunningState {
				break
			}
			if err != nil {
				logging.GetLogger().Errorf("Accept error : %s", err.Error())
				time.Sleep(200 * time.Millisecond)
				continue
			}

			s.handleFlowPacket(conn)
		}

		s.wgFlowsHandlers.Wait()
	}()
}

// Stop the server
func (s *FlowServer) Stop() {
	if atomic.CompareAndSwapInt64(&s.state, common.RunningState, common.StoppingState) {
		s.conn.Cleanup()
		s.wgServer.Wait()
	}
}

// NewFlowServer creates a new flow server listening at address/port, based on configuration
func NewFlowServer(addr string, port int, g *graph.Graph, store storage.Storage, probe *probe.ProbeBundle) (*FlowServer, error) {
	cache := cache.New(time.Duration(600)*time.Second, time.Duration(600)*time.Second)
	pipeline := flow.NewEnhancerPipeline(enhancers.NewGraphFlowEnhancer(g, cache))

	// check that the neutron probe is loaded if so add the neutron flow enhancer
	if probe.GetProbe("neutron") != nil {
		pipeline.AddEnhancer(enhancers.NewNeutronFlowEnhancer(g, cache))
	}

	bulk := config.GetConfig().GetInt("analyzer.storage.bulk_insert")
	deadline := config.GetConfig().GetInt("analyzer.storage.bulk_insert_deadline")

	return &FlowServer{
		Addr:             addr,
		Port:             port,
		Storage:          store,
		EnhancerPipeline: pipeline,
		bulkInsert:       bulk,
		bulkDeadline:     deadline,
	}, nil
}
