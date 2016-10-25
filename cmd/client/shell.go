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

package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
)

var ShellCmd = &cobra.Command{
	Use:          "shell",
	Short:        "Shell Command Line Interface",
	Long:         "Skydive Shell Command Line Interface, yet another shell",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		shellMain()
	},
}

var (
	ErrContinue = errors.New("<continue input>")
	ErrQuit     = errors.New("<quit session>")
)

func actionGremlinQuery(s *Session, query string) error {
	body, err := SendGremlinQuery(&s.authenticationOpts, query)
	if err != nil {
		return err
	}

	var values interface{}

	decoder := json.NewDecoder(body)
	decoder.UseNumber()

	err = decoder.Decode(&values)
	if err != nil {
		return err
	}

	printJSON(values)
	return nil
}

func actionSetVarUsername(s *Session, arg string) error {
	s.authenticationOpts.Username = arg
	return nil
}
func actionSetVarPassword(s *Session, arg string) error {
	s.authenticationOpts.Password = arg
	return nil
}
func actionSetVarAnalyzer(s *Session, arg string) error {
	s.analyzerAddr = arg
	config.GetConfig().Set("agent.analyzers", s.analyzerAddr)
	return nil
}

var vocaGremlinBase = []string{
	"V(",
	"Context(",
	"Flows(",
}

var vocaGremlinExt = []string{
	"Has(",
	"Dedup()",
	"ShortestPathTo(", // 1 or 2
	"Both()",
	"Count()",
	"Range(", // 2
	"Limit(", // 1
	"Sort(",
	"Out()",
	"OutV()",
	"OutE()",
	"In()",
	"InV()",
	"InE()",
}

func completeG(s *Session, prefix string) []string {
	if prefix == "" {
		return vocaGremlinBase
	}
	return vocaGremlinExt
}

type command struct {
	name     string
	action   func(*Session, string) error
	complete func(*Session, string) []string
	arg      string
	document string
}

var commands = []command{
	{
		name:     "g",
		action:   actionGremlinQuery,
		complete: completeG,
		arg:      "<gremlin expression>",
		document: "evaluate a gremlin expression",
	},
	{
		name:     "username",
		action:   actionSetVarUsername,
		complete: nil,
		arg:      "<username>",
		document: "set the analyzer connection username",
	},
	{
		name:     "password",
		action:   actionSetVarPassword,
		complete: nil,
		arg:      "<password>",
		document: "set the analyzer connection password",
	},
	{
		name:     "analyzer",
		action:   actionSetVarAnalyzer,
		complete: nil,
		arg:      "<address:port>",
		document: "set the analyzer connection address",
	},
}

func (s *Session) completeWord(line string, pos int) (string, []string, string) {
	if strings.HasPrefix(line, "g") {
		// complete commands
		if !strings.Contains(line[0:pos], ".") {
			pre, post := line[0:pos], line[pos:]
			result := []string{}
			for _, command := range commands {
				name := command.name
				if strings.HasPrefix(name, pre) {
					// having complete means that this command takes an argument (for now)
					if !strings.HasPrefix(post, ".") && command.arg != "" {
						name = name + "."
					}
					result = append(result, name)
				}
			}
			return "", result, post
		}

		// complete command arguments
		for _, command := range commands {
			if command.complete == nil {
				continue
			}

			cmdPrefix := command.name + "."
			if strings.HasPrefix(line, cmdPrefix) && pos >= len(cmdPrefix) {
				complLine := ""
				if len(line)-len(cmdPrefix) > 0 {
					complLine = line[len(cmdPrefix) : len(line)-len(cmdPrefix)]
				}
				return line, command.complete(s, complLine), ""
			}
		}

		return "", nil, ""
	}
	if strings.HasPrefix(line, ":") {
		// complete commands
		if !strings.Contains(line[0:pos], " ") {
			pre, post := line[0:pos], line[pos:]

			result := []string{}
			for _, command := range commands {
				name := ":" + command.name
				if strings.HasPrefix(name, pre) {
					// having complete means that this command takes an argument (for now)
					if !strings.HasPrefix(post, " ") && command.arg != "" {
						name = name + " "
					}
					result = append(result, name)
				}
			}
			return "", result, post
		}

		// complete command arguments
		for _, command := range commands {
			if command.complete == nil {
				continue
			}

			cmdPrefix := ":" + command.name + " "
			if strings.HasPrefix(line, cmdPrefix) && pos >= len(cmdPrefix) {
				return cmdPrefix, command.complete(s, line[len(cmdPrefix):pos]), ""
			}
		}

		return "", nil, ""
	}
	return "", nil, ""
}

func shellMain() {
	s, err := NewSession()
	if err != nil {
		panic(err)
	}

	rl := newContLiner()
	defer rl.Close()

	var historyFile string
	home, err := homeDir()
	if err != nil {
		logging.GetLogger().Errorf("home: %s", err)
	} else {
		historyFile = filepath.Join(home, "history")

		f, err := os.Open(historyFile)
		if err != nil {
			if !os.IsNotExist(err) {
				logging.GetLogger().Errorf("%s", err)
			}
		} else {
			_, err := rl.ReadHistory(f)
			if err != nil {
				logging.GetLogger().Errorf("while reading history: %s", err)
			}
		}
	}

	rl.SetWordCompleter(s.completeWord)

	for {
		in, err := rl.Prompt()
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Fprintf(os.Stderr, "fatal: %s", err)
			os.Exit(1)
		}

		if in == "" {
			continue
		}

		if err := rl.Reindent(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %s\n", err)
			rl.Clear()
			continue
		}

		err = s.Eval(in)
		if err != nil {
			if err == ErrContinue {
				continue
			} else if err == ErrQuit {
				break
			}
			fmt.Println(err)
		}
		rl.Accepted()
	}

	if historyFile != "" {
		err := os.MkdirAll(filepath.Dir(historyFile), 0755)
		if err != nil {
			logging.GetLogger().Errorf("%s", err)
		} else {
			f, err := os.Create(historyFile)
			if err != nil {
				logging.GetLogger().Errorf("%s", err)
			} else {
				_, err := rl.WriteHistory(f)
				if err != nil {
					logging.GetLogger().Errorf("while saving history: %s", err)
				}
			}
		}
	}
}

func homeDir() (home string, err error) {
	home = os.Getenv("SKYDIVE_HOME")
	if home != "" {
		return
	}

	home, err = homedir.Dir()
	if err != nil {
		return
	}

	home = filepath.Join(home, ".skydive")
	return
}

type Session struct {
	authenticationOpts shttp.AuthenticationOpts
	analyzerAddr       string
}

func NewSession() (*Session, error) {
	s := &Session{
		analyzerAddr: "localhost:8082",
		authenticationOpts: shttp.AuthenticationOpts{
			Username: "admin",
			Password: "password",
		},
	}
	config.GetConfig().Set("agent.analyzers", s.analyzerAddr)

	return s, nil
}

func (s *Session) Eval(in string) error {
	logging.GetLogger().Debugf("eval >>> %q", in)
	for _, command := range commands {
		if command.name == "g" && strings.HasPrefix(in, command.name) {
			err := command.action(s, in)
			if err != nil {
				logging.GetLogger().Errorf("%s: %s", command.name, err)
			}
			return nil
		}
		arg := strings.TrimPrefix(in, ":"+command.name)
		if arg == in {
			continue
		}

		if arg == "" || strings.HasPrefix(arg, " ") {
			arg = strings.TrimSpace(arg)
			err := command.action(s, arg)
			if err != nil {
				if err == ErrQuit {
					return err
				}
				logging.GetLogger().Errorf("%s: %s", command.name, err)
			}
			return nil
		}
	}

	return nil
}
