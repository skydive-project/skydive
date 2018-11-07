/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package common

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
)

// SetupTLSLoadCA creates an X509 certificate from file
func SetupTLSLoadCA(certPEM string) (*x509.CertPool, error) {
	rootPEM, err := ioutil.ReadFile(certPEM)
	if err != nil {
		return nil, fmt.Errorf("Failed to open root certificate '%s' : %s", certPEM, err)
	}
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM([]byte(rootPEM))
	if !ok {
		return nil, fmt.Errorf("Failed to parse root certificate '%s'", rootPEM)
	}
	return roots, nil
}

// SetupTLSClientConfig creates a client X509 certificate from public and private key
func SetupTLSClientConfig(certPEM string, keyPEM string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, fmt.Errorf("Can't read X509 key pair set in config : cert '%s' key '%s' : %s", certPEM, keyPEM, err)
	}
	cfgTLS := &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	return cfgTLS, nil
}

// SetupTLSServerConfig creates a server X509 certificate from public and private key
func SetupTLSServerConfig(certPEM string, keyPEM string) (*tls.Config, error) {
	cfgTLS, err := SetupTLSClientConfig(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	cfgTLS.MinVersion = tls.VersionTLS12
	cfgTLS.ClientAuth = tls.VerifyClientCertIfGiven //tls.NoClientCert // tls.RequireAndVerifyClientCert
	cfgTLS.CurvePreferences = []tls.CurveID{tls.CurveP521, tls.CurveP384, tls.CurveP256}
	cfgTLS.PreferServerCipherSuites = true
	cfgTLS.CipherSuites = []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	}

	return cfgTLS, nil
}
