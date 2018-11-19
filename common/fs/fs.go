/*
 * Copyright (C) 2018 Samsung, Inc.
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

package fs

import (
	"bytes"
	"github.com/skydive-project/skydive/logging"
	"io/ioutil"
	"sync"
	"time"
)

// WatcherCallback described the interface for callback on matched event
type WatcherCallback = func(details Event)

// Op type of event occured with the file (mimics fsnotify.Op for
// interface compatibility)
type Op uint32

// Event mimics fsnotify.Event for backward compatibility + passes
// information about file
type Event struct {
	Name     string
	Op       Op
	FileInfo FileInfo
}

// SysProcFsWatcher is an implementation of a wrapper around fs events
type SysProcFsWatcher struct {
	sync.Mutex
	eventHandlers map[Op]WatcherCallback
	fileInfo      map[string]FileInfo
	stopped       chan bool
}

// FileInfo is a structure which describes file being tracked
type FileInfo struct {
	Path    string
	Content []byte
}

// set of constants, type of events might happen with file
const (
	ChangedContent Op = 1 << iota
)

// OnEvent registers handler for specific type of event, simplifies life for developer
func (w *SysProcFsWatcher) OnEvent(eventType Op, cb WatcherCallback) {
	w.Lock()
	defer w.Unlock()
	logging.GetLogger().Debugf("Register event handler: %d", eventType)
	w.eventHandlers[eventType] = cb
}

// AddPath starts tracking changes for new path
func (w *SysProcFsWatcher) AddPath(path string) error {
	w.Lock()
	defer w.Unlock()
	if _, tracked := w.fileInfo[path]; tracked {
		return nil
	}
	logging.GetLogger().Debugf("track changes for the file: %s", path)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	w.fileInfo[path] = FileInfo{
		Path:    path,
		Content: data,
	}
	return nil
}

// RemovePath stops tracking changes for the file
func (w *SysProcFsWatcher) RemovePath(path string) {
	logging.GetLogger().Debugf("stop tracking changes for the file: %s", path)
	w.Lock()
	defer w.Unlock()
	delete(w.fileInfo, path)
}

func (w *SysProcFsWatcher) checkFileContentEvents() {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-w.stopped:
			return
		case <-ticker.C:
			w.Lock()
			for filePath, fileInfo := range w.fileInfo {
				data, err := ioutil.ReadFile(filePath)
				if err != nil {
					logging.GetLogger().Errorf("Error while reading content of the file: %s, Err: %s", filePath, err)
				}
				if bytes.Equal(data, fileInfo.Content) {
					continue
				}
				fileInfo.Content = data
				w.fileInfo[filePath] = fileInfo
				eventHandler, ok := w.eventHandlers[ChangedContent]
				if !ok {
					continue
				}
				eventHandler(Event{
					Name:     filePath,
					Op:       ChangedContent,
					FileInfo: fileInfo,
				})
			}
			w.Unlock()
		}
	}
}

// Start starts the observer
func (w *SysProcFsWatcher) Start() {
	go w.checkFileContentEvents()
}

// Stop stops the observer
func (w *SysProcFsWatcher) Stop() {
	w.stopped <- true
}

// NewSysProcFsWatcher makes new Watcher
func NewSysProcFsWatcher() *SysProcFsWatcher {
	watcher := &SysProcFsWatcher{
		eventHandlers: make(map[Op]WatcherCallback),
		fileInfo:      make(map[string]FileInfo),
		stopped:       make(chan bool),
	}
	return watcher
}
