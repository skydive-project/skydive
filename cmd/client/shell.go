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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

	"github.com/skydive-project/skydive/api/client"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/js"
	"github.com/skydive-project/skydive/graffiti/logging"
)

var (
	shellScript string

	// ErrContinue parser error continue input
	ErrContinue = errors.New("<continue input>")
	// ErrQuit parser error quit session
	ErrQuit = errors.New("<quit session>")
)

// Session describes a shell session
type Session struct {
	authenticationOpts shttp.AuthenticationOpts
	runtime            *js.Runtime
	rl                 *contLiner
	historyFile        string
}

func (s *Session) completeWord(line string, pos int) (string, []string, string) {
	if len(line) == 0 || pos == 0 {
		return "", nil, ""
	}

	// Chunck data to relevant part for autocompletion
	// E.g. in case of nested lines eth.getBalance(eth.coinb<tab><tab>
	start := pos - 1
	for ; start > 0; start-- {
		// Skip all methods and namespaces (i.e. including the dot)
		if line[start] == '.' || (line[start] >= 'a' && line[start] <= 'z') || (line[start] >= 'A' && line[start] <= 'Z') {
			continue
		}
		// We've hit an unexpected character, autocomplete form here
		start++
		break
	}
	return line[:start], s.runtime.CompleteKeywords(line[start:pos]), line[pos:]
}

func (s *Session) loadHistory() error {
	historyFile := ""
	home, err := homeDir()
	if err != nil {
		return fmt.Errorf("Failed to retrieve home directory: %s", err)
	}

	historyFile = filepath.Join(home, "history")
	f, err := os.Open(historyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		_, err := s.rl.ReadHistory(f)
		if err != nil {
			return err
		}
	}

	s.historyFile = historyFile
	return nil
}

func (s *Session) saveHistory() error {
	if s.historyFile == "" {
		return nil
	}

	err := os.MkdirAll(filepath.Dir(s.historyFile), 0750)
	if err != nil {
		return err
	}

	f, err := os.Create(s.historyFile)
	if err != nil {
		return err
	}

	_, err = s.rl.WriteHistory(f)
	return err
}

func (s *Session) prompt() error {
	for {
		in, err := s.rl.Prompt()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if in == "" {
			continue
		}

		err = s.eval(in)
		if err != nil {
			if err == ErrContinue {
				continue
			} else if err == ErrQuit {
				break
			}
			fmt.Println(err)
		}
		s.rl.Accepted()
	}

	return nil
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

// NewSession creates a new shell session
func NewSession() (*Session, error) {
	runtime, err := js.NewRuntime()
	if err != nil {
		return nil, err
	}

	s := &Session{
		authenticationOpts: AuthenticationOpts,
		runtime:            runtime,
		rl:                 newContLiner(),
	}

	client, err := client.NewCrudClientFromConfig(&AuthenticationOpts)
	if err != nil {
		return nil, err
	}

	s.runtime.Start()
	s.runtime.RegisterAPIClient(client)

	if err := s.loadHistory(); err != nil {
		return nil, fmt.Errorf("while reading history: %s", err)
	}

	s.rl.SetWordCompleter(s.completeWord)

	return s, nil
}

// Eval evaluation a input expression
func (s *Session) eval(in string) error {
	_, err := s.runtime.Exec(in)
	if err != nil {
		return fmt.Errorf("Error while executing Javascript '%s': %s", in, err.Error())
	}
	return nil
}

// Close the session
func (s *Session) Close() error {
	return s.rl.Close()
}

// ShellCmd skydive shell root command
var ShellCmd = &cobra.Command{
	Use:          "shell",
	Short:        "Shell Command Line Interface",
	Long:         "Skydive Shell Command Line Interface, yet another shell",
	SilenceUsage: false,
	Run: func(cmd *cobra.Command, args []string) {
		s, err := NewSession()
		if err != nil {
			logging.GetLogger().Errorf("Error while creating session: %s", err)
			os.Exit(1)
		}
		defer s.Close()

		if shellScript != "" {
			result := s.runtime.RunScript(shellScript)
			if result.IsDefined() {
				logging.GetLogger().Errorf("Error while executing script %s: %s", shellScript, result.String())
			}
		}

		s.prompt()

		if err := s.saveHistory(); err != nil {
			logging.GetLogger().Errorf("Error while saving history: %s", err)
		}
	},
}

func init() {
	ShellCmd.Flags().StringVarP(&shellScript, "script", "", "", "path to a JavaScript to execute")
}
