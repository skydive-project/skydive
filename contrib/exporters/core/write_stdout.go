/*
 * Copyright (C) 2019 IBM, Inc.
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

package core

import (
	"fmt"

	"github.com/spf13/viper"
)

type writeStdout struct {
}

// Write stores a single file
func (s *writeStdout) Write(dirname, filename, content, contentType, contentEncoding string,
	metadata map[string]*string) error {
	fmt.Printf("--- %s/%s ---\n", dirname, filename)
	fmt.Printf("%s\n", content)
	return nil
}

// NewWriteStdout returns a new storage interface for storing flows to object store
func NewWriteStdout(cfg *viper.Viper) (interface{}, error) {
	return &writeStdout{}, nil
}
