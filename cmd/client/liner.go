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
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/scanner"

	"github.com/peterh/liner"
	"github.com/skydive-project/skydive/logging"
)

const (
	promptDefault  = "skydive> "
	promptContinue = "..... "
	indent         = "    "
)

type contLiner struct {
	*liner.State
	buffer string
	depth  int
}

func newContLiner() *contLiner {
	rl := liner.NewLiner()
	rl.SetCtrlCAborts(true)
	return &contLiner{State: rl}
}

func (cl *contLiner) promptString() string {
	if cl.buffer != "" {
		return promptContinue + strings.Repeat(indent, cl.depth)
	}

	return promptDefault
}

func (cl *contLiner) Prompt() (string, error) {
	line, err := cl.State.Prompt(cl.promptString())
	if err == io.EOF {
		if cl.buffer != "" {
			// cancel line continuation
			cl.Accepted()
			fmt.Println()
			err = nil
		}
	} else if err == liner.ErrPromptAborted {
		err = nil
		if cl.buffer != "" {
			cl.Accepted()
		} else {
			fmt.Println("(^D to quit)")
		}
	} else if err == nil {
		if cl.buffer != "" {
			cl.buffer = cl.buffer + "\n" + line
		} else {
			cl.buffer = line
		}
	}

	return cl.buffer, err
}

func (cl *contLiner) Accepted() {
	cl.State.AppendHistory(cl.buffer)
	cl.buffer = ""
}

func (cl *contLiner) Clear() {
	cl.buffer = ""
	cl.depth = 0
}

var errUnmatchedBraces = fmt.Errorf("unmatched braces")

func (cl *contLiner) Reindent() error {
	oldDepth := cl.depth
	cl.depth = cl.countDepth()

	if cl.depth < 0 {
		return errUnmatchedBraces
	}

	if cl.depth < oldDepth {
		lines := strings.Split(cl.buffer, "\n")
		if len(lines) > 1 {
			lastLine := lines[len(lines)-1]

			cursorUp()
			fmt.Printf("\r%s%s", cl.promptString(), lastLine)
			eraseInLine()
			fmt.Print("\n")
		}
	}

	return nil
}

func (cl *contLiner) countDepth() int {
	reader := bytes.NewBufferString(cl.buffer)
	sc := new(scanner.Scanner)
	sc.Init(reader)
	sc.Error = func(_ *scanner.Scanner, msg string) {
		logging.GetLogger().Debugf("scanner: %s", msg)
	}

	depth := 0
	for {
		switch sc.Scan() {
		case '{':
			depth++
		case '}':
			depth--
		case scanner.EOF:
			return depth
		}
	}
}
