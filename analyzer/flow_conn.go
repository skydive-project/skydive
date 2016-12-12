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
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

var ErrFlowUDPAcceptNotSupported = errors.New("UDP connection is datagram based (not connected), accept() not supported")

type FlowConnectionType int

const (
	UDP FlowConnectionType = 1 + iota
	TLS
)

type FlowServerConn struct {
	mode      FlowConnectionType
	udpConn   *net.UDPConn
	tlsConn   net.Conn
	tlsListen net.Listener
}

func (a *FlowServerConn) Mode() FlowConnectionType {
	return a.mode
}

func (a *FlowServerConn) Accept() (*FlowServerConn, error) {
	switch a.mode {
	case TLS:
		acceptedTLSConn, err := a.tlsListen.Accept()
		if err != nil {
			return nil, err
		}

		tlsConn, ok := acceptedTLSConn.(*tls.Conn)
		if !ok {
			logging.GetLogger().Fatalf("This is not a TLS connection %v", a.tlsConn)
		}
		err = tlsConn.Handshake()
		if err != nil {
			return nil, err
		}
		state := tlsConn.ConnectionState()
		if state.HandshakeComplete == false {
			return nil, errors.New("TLS Handshake is not complete")
		}
		return &FlowServerConn{
			mode:    TLS,
			tlsConn: acceptedTLSConn,
		}, nil
	case UDP:
		return a, ErrFlowUDPAcceptNotSupported
	}
	return nil, errors.New("Connection mode is not set properly")
}

func (a *FlowServerConn) Cleanup() {
	if a.mode == TLS {
		err := a.tlsListen.Close()
		if err != nil {
			logging.GetLogger().Errorf("Close error %v", err)
		}
	}
}

func (a *FlowServerConn) Close() {
	switch a.mode {
	case TLS:
		err := a.tlsConn.Close()
		if err != nil {
			logging.GetLogger().Errorf("Close error %v", err)
		}
	case UDP:
		err := a.udpConn.Close()
		if err != nil {
			logging.GetLogger().Errorf("Close error %v", err)
		}
	}
}

func (a *FlowServerConn) SetDeadline(t time.Time) {
	switch a.mode {
	case TLS:
		err := a.tlsConn.SetReadDeadline(t)
		if err != nil {
			logging.GetLogger().Errorf("SetReadDeadline %v", err)
		}
	case UDP:
		err := a.udpConn.SetDeadline(t)
		if err != nil {
			logging.GetLogger().Errorf("SetDeadline %v", err)
		}
	}
}

func (a *FlowServerConn) Read(data []byte) (int, error) {
	switch a.mode {
	case TLS:
		n, err := a.tlsConn.Read(data)
		return n, err
	case UDP:
		n, _, err := a.udpConn.ReadFromUDP(data)
		return n, err
	}
	return 0, errors.New("Mode didn't exist")
}

func (a *FlowServerConn) Timeout(err error) bool {
	switch a.mode {
	case TLS:
		if netErr, ok := err.(net.Error); ok {
			return netErr.Timeout()
		}
	case UDP:
		if netErr, ok := err.(*net.OpError); ok {
			return netErr.Timeout()
		}
	}
	return false
}

func NewFlowServerConn(addr *net.UDPAddr) (a *FlowServerConn, err error) {
	a = &FlowServerConn{mode: UDP}
	certPEM := config.GetConfig().GetString("analyzer.X509_cert")
	keyPEM := config.GetConfig().GetString("analyzer.X509_key")
	clientCertPEM := config.GetConfig().GetString("agent.X509_cert")

	if len(certPEM) > 0 && len(keyPEM) > 0 {
		cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
		if err != nil {
			logging.GetLogger().Fatalf("Can't read X509 key pair set in config : cert '%s' key '%s'", certPEM, keyPEM)
		}
		rootPEM, err := ioutil.ReadFile(clientCertPEM)
		if err != nil {
			logging.GetLogger().Fatalf("Failed to open root certificate '%s' : %s", certPEM, err.Error())
		}
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM([]byte(rootPEM))
		if !ok {
			logging.GetLogger().Fatal("Failed to parse root certificate " + certPEM)
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
		a.tlsListen, err = tls.Listen("tcp", fmt.Sprintf("%s:%d", common.IPToString(addr.IP), addr.Port+1), cfgTLS)
		if err != nil {
			return nil, err
		}
		a.mode = TLS
		logging.GetLogger().Info("Analyzer listen agents on TLS socket")
		return a, nil
	}
	a.udpConn, err = net.ListenUDP("udp", addr)
	logging.GetLogger().Info("Analyzer listen agents on UDP socket")
	return a, err
}

type FlowClientConn struct {
	udpConn       *net.UDPConn
	tlsConnClient *tls.Conn
}

func (a *FlowClientConn) Close() {
	if a.tlsConnClient != nil {
		err := a.tlsConnClient.Close()
		if err != nil {
			logging.GetLogger().Errorf("Close error %v", err)
		}
		return
	}
	err := a.udpConn.Close()
	if err != nil {
		logging.GetLogger().Errorf("Close error %v", err)
	}
}

func (a *FlowClientConn) Write(b []byte) (int, error) {
	if a.tlsConnClient != nil {
		return a.tlsConnClient.Write(b)
	}
	return a.udpConn.Write(b)
}

func NewFlowClientConn(addr *net.UDPAddr) (a *FlowClientConn, err error) {
	a = &FlowClientConn{}
	certPEM := config.GetConfig().GetString("agent.X509_cert")
	keyPEM := config.GetConfig().GetString("agent.X509_key")
	serverCertPEM := config.GetConfig().GetString("analyzer.X509_cert")

	if len(certPEM) > 0 && len(keyPEM) > 0 {
		cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
		if err != nil {
			logging.GetLogger().Fatalf("Can't read X509 key pair set in config : cert '%s' key '%s'", certPEM, keyPEM)
			return nil, err
		}
		rootPEM, err := ioutil.ReadFile(serverCertPEM)
		if err != nil {
			logging.GetLogger().Fatalf("Failed to open root certificate '%s' : %s", certPEM, err.Error())
		}
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(rootPEM)
		if !ok {
			logging.GetLogger().Fatal("Failed to parse root certificate " + certPEM)
		}
		cfgTLS := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      roots,
		}
		cfgTLS.BuildNameToCertificate()
		logging.GetLogger().Debugf("TLS client connection ... Dial %s:%d", common.IPToString(addr.IP), addr.Port+1)
		a.tlsConnClient, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", common.IPToString(addr.IP), addr.Port+1), cfgTLS)
		if err != nil {
			logging.GetLogger().Errorf("TLS error %s:%d : %s", common.IPToString(addr.IP), addr.Port+1, err.Error())
			return nil, err
		}
		state := a.tlsConnClient.ConnectionState()
		if state.HandshakeComplete == false {
			logging.GetLogger().Debugf("TLS Handshake is not complete %s:%d : %+v", common.IPToString(addr.IP), addr.Port+1, state)
			return nil, errors.New("TLS Handshake is not complete")
		}
		logging.GetLogger().Debugf("TLS v%d Handshake is complete on %s:%d", state.Version, common.IPToString(addr.IP), addr.Port+1)
		return a, nil
	}
	a.udpConn, err = net.DialUDP("udp", nil, addr)
	if err != nil {
		return nil, err
	}
	logging.GetLogger().Debugf("UDP client dialup done for: %s:%d", addr.IP, addr.Port)
	return a, nil
}
