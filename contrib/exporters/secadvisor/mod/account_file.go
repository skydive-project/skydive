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

package mod

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"github.com/spf13/viper"
)

func makeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

type accountFile struct {
	filename string
}

type accountRecord struct {
	StartupTime  int64 `json:"startup-time"`
	StartupBytes int64 `json:"startup-bytes"`
	UpdateTime   int64 `json:"update-time"`
	UpdateBytes  int64 `json:"update-bytes"`
}

func (a *accountFile) read(filename string) *accountRecord {
	record := &accountRecord{
		StartupTime: makeTimestamp(),
	}

	file, err := os.Open(filename)
	if err != nil {
		return record
	}
	defer file.Close()

	content, _ := ioutil.ReadAll(file)
	json.Unmarshal(content, &record)
	return record
}

func (a *accountFile) write(filename string, record *accountRecord) {
	content, _ := json.MarshalIndent(record, "", " ")
	ioutil.WriteFile(filename, content, 0644)
}

func (a *accountFile) bakfile() string {
	return a.filename + ".bak"
}

// Reset counters
func (a *accountFile) Reset() {
	os.Remove(a.filename)
	os.Remove(a.bakfile())
}

// Add to counters
func (a *accountFile) Add(bytes int64) {
	record := a.read(a.filename)
	if record.StartupTime == 0 {
		record = a.read(a.bakfile())
	}
	record.StartupBytes += bytes
	record.UpdateTime = makeTimestamp()
	record.UpdateBytes += bytes
	a.write(a.bakfile(), record)
	os.Remove(a.filename)
	os.Rename(a.bakfile(), a.filename)
}

// NewAccountNone create a new accounter
func NewAccountFile(cfg *viper.Viper) (interface{}, error) {
	return &accountFile{}, nil
}
