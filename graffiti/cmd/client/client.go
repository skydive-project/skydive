/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package client

import (
	"crypto/tls"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
	gtls "github.com/skydive-project/skydive/graffiti/tls"
)

const (
	CLIService service.Type = "cli"
)

var (
	// AuthenticationOpts Authentication options
	AuthenticationOpts http.AuthenticationOpts
	// CrudClient holds an API CRUD client
	CrudClient *http.CrudClient
	// Host holds the hostname
	Host string
	// HubAddress holds the hub address
	HubAddress string

	hubService service.Address
	tlsConfig  *tls.Config
	cafile     string
	certfile   string
	keyfile    string
	skipVerify bool
)

// ClientCmd describes the skydive client root command
var ClientCmd = &cobra.Command{
	Use:          "client",
	Short:        "Skydive client",
	Long:         "Skydive client",
	SilenceUsage: true,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		var err error
		hubService, err = service.AddressFromString(HubAddress)
		if err != nil {
			exitOnError(err)
		}

		if certfile != "" && keyfile != "" {
			tlsConfig, err = gtls.SetupTLSClientConfig(certfile, keyfile)
			if err != nil {
				exitOnError(err)
			}

			tlsConfig.InsecureSkipVerify = skipVerify

			if cafile != "" {
				tlsConfig.RootCAs, err = gtls.SetupTLSLoadCA(cafile)
				if err != nil {
					exitOnError(err)
				}
			}
		}

		url, _ := http.MakeURL("http", hubService.Addr, hubService.Port, "/api/", tlsConfig != nil)
		restClient := http.NewRestClient(url, &AuthenticationOpts, tlsConfig)
		CrudClient = http.NewCrudClient(restClient)
	},
}

func exitOnError(err error) {
	logging.GetLogger().Error(err)
	os.Exit(1)
}

// DefToMetadata converts a string in k1=v1,k2=v2,... format to a metadata object
func DefToMetadata(def string, metadata graph.Metadata) (graph.Metadata, error) {
	if def == "" {
		return metadata, nil
	}

	for _, pair := range strings.Split(def, ",") {
		pair = strings.TrimSpace(pair)

		kv := strings.Split(pair, "=")
		if len(kv)%2 != 0 {
			return nil, fmt.Errorf("attributes must be defined by pair k=v: %v", def)
		}
		key := strings.Trim(kv[0], `"`)
		value := strings.Trim(kv[1], `"`)

		metadata.SetField(key, value)
	}

	return metadata, nil
}

func init() {
	ClientCmd.PersistentFlags().StringVarP(&AuthenticationOpts.Username, "username", "", AuthenticationOpts.Username, "username auth parameter")
	ClientCmd.PersistentFlags().StringVarP(&AuthenticationOpts.Password, "password", "", AuthenticationOpts.Password, "password auth parameter")

	ClientCmd.PersistentFlags().StringVarP(&cafile, "cacert", "", "", "certificate file to verify the peer")
	ClientCmd.PersistentFlags().StringVarP(&certfile, "cert", "", "", "client certificate file")
	ClientCmd.PersistentFlags().StringVarP(&keyfile, "key", "", "", "private key file name")
	ClientCmd.PersistentFlags().BoolVarP(&skipVerify, "insecure", "", false, "do not check server's certificate")

	ClientCmd.AddCommand(AlertCmd)
	ClientCmd.AddCommand(QueryCmd)
	ClientCmd.AddCommand(EdgeCmd)
	ClientCmd.AddCommand(NodeCmd)
	ClientCmd.AddCommand(ShellCmd)
	ClientCmd.AddCommand(TopologyCmd)
	ClientCmd.AddCommand(WorkflowCmd)
}
