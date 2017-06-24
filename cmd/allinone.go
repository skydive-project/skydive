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

package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/osext"
	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

var (
	analyzerUID uint32
)

// AllInOne skydive allinone root command
var AllInOne = &cobra.Command{
	Use:          "allinone",
	Short:        "Skydive All-In-One",
	Long:         "Skydive All-In-One (Analyzer + Agent)",
	SilenceUsage: true,
	Run: func(cmd *cobra.Command, args []string) {
		skydivePath, _ := osext.Executable()

		analyzerAttr := &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			Env:   os.Environ(),
		}

		if analyzerUID != 0 {
			analyzerAttr.Sys = &syscall.SysProcAttr{
				Credential: &syscall.Credential{
					Uid: analyzerUID,
				},
			}
		}

		args = []string{"skydive"}

		for _, cfgFile := range cfgFiles {
			args = append(args, "-c")
			args = append(args, cfgFile)
		}

		if cfgBackend != "" {
			args = append(args, "-b")
			args = append(args, cfgBackend)
		}

		analyzerArgs := make([]string, len(args))
		copy(analyzerArgs, args)

		if len(cfgFiles) != 0 {
			if err := config.InitConfig(cfgBackend, cfgFiles); err != nil {
				panic(fmt.Sprintf("Failed to initialize config: %s", err.Error()))
			}
		}

		if err := logging.InitLogging(); err != nil {
			panic(fmt.Sprintf("Failed to initialize logging system: %s", err.Error()))
		}

		analyzer, err := os.StartProcess(skydivePath, append(analyzerArgs, "analyzer"), analyzerAttr)
		if err != nil {
			logging.GetLogger().Fatalf("Can't start Skydive All-in-One : %v", err)
		}

		os.Setenv("SKYDIVE_ANALYZERS", config.GetConfig().GetString("analyzer.listen"))

		agentAttr := &os.ProcAttr{
			Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
			Env:   os.Environ(),
		}

		agentArgs := make([]string, len(args))
		copy(agentArgs, args)

		agent, err := os.StartProcess(skydivePath, append(agentArgs, "agent"), agentAttr)
		if err != nil {
			logging.GetLogger().Fatalf("Can't start Skydive All-in-One : %v", err)
		}

		logging.GetLogger().Notice("Skydive All-in-One starting !")
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch

		analyzer.Kill()
		agent.Kill()

		analyzer.Wait()
		agent.Wait()

		logging.GetLogger().Notice("Skydive All-in-One stopped.")
	},
}

func init() {
	var uid uint32

	skydivePath, _ := osext.Executable()

	fi, err := os.Stat(skydivePath)
	if err == nil {
		uid = fi.Sys().(*syscall.Stat_t).Uid
	}

	AllInOne.Flags().Uint32VarP(&analyzerUID, "analyzer-uid", "", uid, "uid used for analyzer process")
}
