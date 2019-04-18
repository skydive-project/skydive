/*
 * Copyright (C) 2018 Orange
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

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/probes/ovsdb/jsonof"
)

func main() {
	var err error
	var result []interface{}
	var jsBytes []byte
	gFlag := flag.Bool("g", false, "parse input as groups not rules")
	fileFlag := flag.String("f", "-", "name of file to parse")
	prettyFlag := flag.Bool("p", false, "prettyfy the output")
	flag.Parse()
	cin := os.Stdin
	if *fileFlag != "-" {
		cin, err = os.Open(*fileFlag)
		if err != nil {
			logging.GetLogger().Errorf("Cannot open file %s: %s", *fileFlag, err)
			os.Exit(1)
		}
	}
	scanner := bufio.NewScanner(cin)
	for scanner.Scan() {
		line := scanner.Text()
		if *gFlag {
			ast, err2 := jsonof.ToASTGroup(line)
			if err2 != nil {
				logging.GetLogger().Error(err)
				continue
			}
			result = append(result, ast)
		} else {
			ast, err2 := jsonof.ToAST(line)
			if err2 != nil {
				logging.GetLogger().Error(err)
				continue
			}
			result = append(result, ast)
		}
	}
	if *prettyFlag {
		jsBytes, err = json.MarshalIndent(result, "", "  ")
	} else {
		jsBytes, err = json.Marshal(result)
	}
	if err != nil {
		logging.GetLogger().Error(err)
		os.Exit(1)
	}
	fmt.Println(string(jsBytes))
}
