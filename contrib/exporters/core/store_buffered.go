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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/IBM/ibm-cos-sdk-go/aws"

	"github.com/skydive-project/skydive/logging"
)

// stream is a series of consecutive persisted objects
type stream struct {
	// ID holds the time when the stream was created
	ID time.Time
	// SeqNumber holds the next available sequence number of this stream
	SeqNumber int
}

// storeBuffered allows writing flows to an object storage service
type storeBuffered struct {
	dirname           string
	filenamePrefix    string
	currentStream     map[Tag]stream
	maxFlowsPerObject int
	maxFlowArraySize  int
	maxStreamDuration time.Duration
	maxObjectDuration time.Duration
	pipeline          *Pipeline
	flows             map[Tag][]interface{}
	lastFlushTime     map[Tag]time.Time
	flushTimers       map[Tag]*time.Timer
	flowsMutex        sync.Mutex
}

// SetPipeline setup
func (s *storeBuffered) SetPipeline(pipeline *Pipeline) {
	s.pipeline = pipeline
}

// StoreFlows store flows in memory, before being written to the object store
func (s *storeBuffered) StoreFlows(in map[Tag][]interface{}) error {
	s.flowsMutex.Lock()
	defer s.flowsMutex.Unlock()

	endTime := time.Now()

	for t, incomingTagFlows := range in {
		s.flows[t] = append(s.flows[t], incomingTagFlows...)
	}

	// check which flows needs to be flushed to the object store
	flushedTags := make(map[Tag]bool)
	tagsToTimerReset := make(map[Tag]bool)
	for t := range s.flows {
		if _, ok := s.lastFlushTime[t]; !ok {
			s.lastFlushTime[t] = endTime
			tagsToTimerReset[t] = true
		}
		for {
			flowsLength := len(s.flows[t])
			if flowsLength == 0 {
				break
			}
			if flowsLength < s.maxFlowsPerObject && time.Since(s.lastFlushTime[t]) < s.maxObjectDuration {
				break
			}

			if s.flushFlowsToObject(t, endTime) != nil {
				break
			}

			flushedTags[t] = true
			tagsToTimerReset[t] = true
		}

		// check if need to trim flow array
		overflowCount := len(s.flows[t]) - s.maxFlowArraySize
		if overflowCount > 0 {
			logging.GetLogger().Warningf("Flow array for tag '%s' overflowed maximum size. Discarding %d flows",
				string(t), overflowCount)
			s.flows[t] = s.flows[t][overflowCount:]
		}
	}

	for t := range flushedTags {
		// copy slice to free up memory of already flushed flows
		newArray := make([]interface{}, 0, len(s.flows[t]))
		copy(newArray, s.flows[t])
		s.flows[t] = newArray
	}

	for t := range tagsToTimerReset {
		// stop old flush timers
		oldTimer, ok := s.flushTimers[t]
		if ok {
			oldTimer.Stop()
		}
	}

	// set a timer for next flush
	if s.maxObjectDuration > 0 {
		for t := range tagsToTimerReset {
			s.flushTimers[t] = time.AfterFunc(s.maxObjectDuration-time.Since(endTime), func() {
				if err := s.StoreFlows(nil); err != nil {
					logging.GetLogger().Error("failed to store flows: ", err)
				}
			})

			// a single timer will cover all of the flushed tags
			break
		}
	}

	return nil
}

func (s *storeBuffered) flushFlowsToObject(t Tag, endTime time.Time) error {
	flows := s.flows[t]
	if len(flows) == 0 {
		return nil
	}

	if len(flows) > s.maxFlowsPerObject {
		flows = flows[:s.maxFlowsPerObject]
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
		logging.GetLogger().Error("failed to encode object: ", err)
		return err
	}

	b, err := s.pipeline.Compressor.Compress(encodedFlows)
	if err != nil {
		logging.GetLogger().Error("failed to compress object: ", err)
		return err
	}

	keyParts := []string{
		s.filenamePrefix,
		string(t),
		currentStream.ID.UTC().Format("20060102T150405Z"),
		fmt.Sprintf("%08d.gz", currentStream.SeqNumber),
	}
	objectKey := strings.Join(keyParts, "/")
	err = s.pipeline.Writer.Write(s.dirname, objectKey, b.String(), "application/json", "gzip", metadata)

	if err != nil {
		logging.GetLogger().Error("failed to write object: ", err)
		return err
	}

	// flush was successful, update state
	s.lastFlushTime[t] = endTime
	currentStream.SeqNumber++
	s.currentStream[t] = currentStream
	s.flows[t] = s.flows[t][len(flows):]

	return nil
}

// NewStoreBuffered returns a new storage interface for storing flows to object store
func NewStoreBuffered(cfg *viper.Viper) (interface{}, error) {
	dirname := cfg.GetString(CfgRoot + "store.buffered.dirname")
	filenamePrefix := cfg.GetString(CfgRoot + "store.buffered.filename_prefix")
	maxFlowArraySize := cfg.GetInt(CfgRoot + "store.buffered.max_flow_array_size")
	maxFlowsPerObject := cfg.GetInt(CfgRoot + "store.buffered.max_flows_per_object")
	maxSecondsPerObject := cfg.GetInt(CfgRoot + "store.buffered.max_seconds_per_object")
	maxSecondsPerStream := cfg.GetInt(CfgRoot + "store.buffered.max_seconds_per_stream")

	store := &storeBuffered{
		dirname:           dirname,
		filenamePrefix:    filenamePrefix,
		maxFlowsPerObject: maxFlowsPerObject,
		maxFlowArraySize:  maxFlowArraySize,
		maxObjectDuration: time.Second * time.Duration(maxSecondsPerObject),
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		currentStream:     make(map[Tag]stream),
		flows:             make(map[Tag][]interface{}),
		lastFlushTime:     make(map[Tag]time.Time),
		flushTimers:       make(map[Tag]*time.Timer),
	}

	return store, nil
}
