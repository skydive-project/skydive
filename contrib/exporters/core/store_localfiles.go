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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/viper"

	"github.com/IBM/ibm-cos-sdk-go/aws"

	"github.com/skydive-project/skydive/logging"
)

// StoreLocalFiles allows writing flows to a local directory
type StoreLocalFiles struct {
	baseDir           string
	currentStream     map[Tag]stream
	maxStreamDuration time.Duration
	pipeline          *Pipeline
	flows             map[Tag][]interface{}
	lastFlushTime     map[Tag]time.Time
}

// SetPipeline setup
func (s *StoreLocalFiles) SetPipeline(pipeline *Pipeline) {
	s.pipeline = pipeline
}

// StoreFlows store flows in memory, before being written to the file system
func (s *StoreLocalFiles) StoreFlows(in map[Tag][]interface{}) error {
	endTime := time.Now()

	s.flows = in

	// check which flows needs to be flushed to the file system
	flushedTags := make(map[Tag]bool)
	for t := range s.flows {
		if _, ok := s.lastFlushTime[t]; !ok {
			s.lastFlushTime[t] = endTime
		}
		for {
			flowsLength := len(s.flows[t])
			if flowsLength == 0 {
				break
			}
			if s.flushFlowsToFile(t, endTime) != nil {
				break
			}
			flushedTags[t] = true
		}
	}

	for t := range flushedTags {
		// copy slice to free up memory of already flushed flows
		newArray := make([]interface{}, 0, len(s.flows[t]))
		copy(newArray, s.flows[t])
		s.flows[t] = newArray
	}

	return nil
}

func (s *StoreLocalFiles) flushFlowsToFile(t Tag, endTime time.Time) error {
	flows := s.flows[t]
	if len(flows) == 0 {
		return nil
	}

	startTime := s.lastFlushTime[t]
	startTimeString := strconv.FormatInt(startTime.UTC().UnixNano()/int64(time.Millisecond), 10)
	endTimeString := strconv.FormatInt(endTime.UTC().UnixNano()/int64(time.Millisecond), 10)

	metadata := map[string]*string{
		"first-timestamp": aws.String(startTimeString),
		"last-timestamp":  aws.String(endTimeString),
		"num-records":     aws.String(strconv.Itoa(len(flows))),
	}

	currentStream := s.currentStream[t]
	if endTime.Sub(currentStream.ID) >= s.maxStreamDuration {
		currentStream = stream{ID: endTime}
	}

	encodedFlows, err := s.pipeline.Encoder.Encode(flows)
	if err != nil {
		logging.GetLogger().Error("Failed to encode object: ", err)
		return err
	}

	b, err := s.pipeline.Compressor.Compress(encodedFlows)
	if err != nil {
		logging.GetLogger().Error("Failed to compress object: ", err)
		return err
	}

	dir := filepath.Join(s.baseDir, string(t), currentStream.ID.UTC().Format("20060102T150405Z"))
	err = os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		logging.GetLogger().Error("Failed to create directory: ", err)
		return err
	}
	fullPath := filepath.Join(dir, fmt.Sprintf("%08d.gz", currentStream.SeqNumber))
	err = ioutil.WriteFile(fullPath, b.Bytes(), 0644)
	if err != nil {
		logging.GetLogger().Error("Failed to write file: ", err)
		return err
	}
	logging.GetLogger().Info("Wrote flows: ", fullPath)

	metadataBytes, err := json.MarshalIndent(metadata, "", "\t")
	if err != nil {
		logging.GetLogger().Error("Failed marshal metadata: ", err)
		return err
	}
	metadataBytes = append(metadataBytes, '\n')
	metadataFullPath := fullPath + ".metadata.json"
	err = ioutil.WriteFile(metadataFullPath, metadataBytes, 0644)
	if err != nil {
		logging.GetLogger().Error("Failed to write metadata file: ", err)
		return err
	}
	logging.GetLogger().Info("Wrote metadata: ", metadataFullPath)

	// flush was successful, update state
	s.lastFlushTime[t] = endTime
	currentStream.SeqNumber++
	s.currentStream[t] = currentStream
	s.flows[t] = s.flows[t][len(flows):]

	return nil
}

// NewStoreLocalFiles returns a new storage interface for storing flows to a local directory on disk
func NewStoreLocalFiles(cfg *viper.Viper) (interface{}, error) {
	baseDir := cfg.GetString(CfgRoot + "store.localfiles.base_dir")
	if baseDir == "" {
		return nil, fmt.Errorf("Missing configuration store.localfiles.base_dir")
	}
	maxSecondsPerStream := cfg.GetInt(CfgRoot + "store.localfiles.max_seconds_per_stream")

	store := &StoreLocalFiles{
		baseDir:           baseDir,
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		currentStream:     make(map[Tag]stream),
		flows:             make(map[Tag][]interface{}),
		lastFlushTime:     make(map[Tag]time.Time),
	}

	return store, nil
}
