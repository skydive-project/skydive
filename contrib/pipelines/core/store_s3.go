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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"

	"github.com/IBM/ibm-cos-sdk-go/aws"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
)

// stream is a series of consecutive persisted objects
type stream struct {
	// ID holds the time when the stream was created
	ID time.Time
	// SeqNumber holds the next available sequence number of this stream
	SeqNumber int
}

// StoreS3 allows writing flows to an object storage service
type StoreS3 struct {
	bucket            string
	objectPrefix      string
	currentStream     map[Tag]stream
	maxFlowsPerObject int
	maxFlowArraySize  int
	maxStreamDuration time.Duration
	maxObjectDuration time.Duration
	client            objectStoreClient
	pipeline          *Pipeline
	flows             map[Tag][]interface{}
	flowsMutex        sync.Mutex
	lastFlushTime     map[Tag]time.Time
	flushTimers       map[Tag]*time.Timer
}

// SetPipeline setup
func (s *StoreS3) SetPipeline(pipeline *Pipeline) {
	s.pipeline = pipeline
}

// ListObjects lists all stored objects
func (s *StoreS3) ListObjects() ([]*string, error) {
	objectKeys, err := s.client.ListObjects(s.bucket, s.objectPrefix)
	if err != nil {
		logging.GetLogger().Error("Failed to list objects: ", err)
		return nil, err
	}

	return objectKeys, nil
}

// ReadObjectFlows reads flows from object
func (s *StoreS3) ReadObjectFlows(objectKey *string, objectFlows interface{}) error {
	objectBytes, err := s.client.ReadObject(s.bucket, *objectKey)
	if err != nil {
		logging.GetLogger().Error("Failed to read object: ", err)
		return err
	}

	if err = json.Unmarshal(objectBytes, objectFlows); err != nil {
		logging.GetLogger().Error("Failed to JSON-decode object: ", err)
		return err
	}

	return nil
}

// DeleteObject deletes an object
func (s *StoreS3) DeleteObject(objectKey *string) error {
	if err := s.client.DeleteObject(s.bucket, *objectKey); err != nil {
		logging.GetLogger().Error("Failed to delete object: ", err)
		return err
	}
	return nil
}

// StoreFlows store flows in memory, before being written to the object store
func (s *StoreS3) StoreFlows(flows []*flow.Flow) error {
	endTime := time.Now()

	s.flowsMutex.Lock()
	defer s.flowsMutex.Unlock()

	// transform flows and save to in-memory arrays, based on tag
	if flows != nil {
		for _, fl := range flows {
			flowTag := s.pipeline.Classifier.GetFlowTag(fl)
			if s.pipeline.Filterer.IsExcluded(flowTag) {
				continue
			}

			transformedFlow := s.pipeline.Transformer.Transform([]*flow.Flow{fl})

			if transformedFlow != nil {
				s.flows[flowTag] = append(s.flows[flowTag], transformedFlow)
			}
		}
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
			logging.GetLogger().Warningf("Flow array for tag '%s' overflowed maximum size. Discarding %d flows", string(t), overflowCount)
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
				s.StoreFlows(nil)
			})

			// a single timer will cover all of the flushed tags
			break
		}
	}

	return nil
}

func (s *StoreS3) flushFlowsToObject(t Tag, endTime time.Time) error {
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
		logging.GetLogger().Error("Failed to encode object: ", err)
		return err
	}

	b, err := s.pipeline.Compressor.Compress(encodedFlows)
	if err != nil {
		logging.GetLogger().Error("Failed to compress object: ", err)
		return err
	}

	objectKey := strings.Join([]string{s.objectPrefix, string(t), currentStream.ID.UTC().Format("20060102T150405Z"), fmt.Sprintf("%08d.gz", currentStream.SeqNumber)}, "/")
	err = s.client.WriteObject(s.bucket, objectKey, string(b.Bytes()), "application/json", "gzip", metadata)

	if err != nil {
		logging.GetLogger().Error("Failed to write object: ", err)
		return err
	}

	// flush was successful, update state
	s.lastFlushTime[t] = endTime
	currentStream.SeqNumber++
	s.currentStream[t] = currentStream
	s.flows[t] = s.flows[t][len(flows):]

	return nil
}

// NewStoreS3FromConfig returns a new storage interface for storing flows to object store
func NewStoreS3FromConfig(cfg *viper.Viper) (*StoreS3, error) {
	bucket := cfg.GetString(CfgRoot + "store.s3.bucket")
	objectPrefix := cfg.GetString(CfgRoot + "store.s3.object_prefix")
	maxFlowArraySize := cfg.GetInt(CfgRoot + "store.s3.max_flow_array_size")
	maxFlowsPerObject := cfg.GetInt(CfgRoot + "store.s3.max_flows_per_object")
	maxSecondsPerObject := cfg.GetInt(CfgRoot + "store.s3.max_seconds_per_object")
	maxSecondsPerStream := cfg.GetInt(CfgRoot + "store.s3.max_seconds_per_stream")

	client := newClient(cfg)
	return NewStoreS3(client, bucket, objectPrefix, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream)
}

// NewStoreS3 creates a store
func NewStoreS3(client objectStoreClient, bucket, objectPrefix string, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream int) (*StoreS3, error) {
	s := &StoreS3{
		bucket:            bucket,
		objectPrefix:      objectPrefix,
		maxFlowsPerObject: maxFlowsPerObject,
		maxFlowArraySize:  maxFlowArraySize,
		maxObjectDuration: time.Second * time.Duration(maxSecondsPerObject),
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		client:            client,
		currentStream:     make(map[Tag]stream),
		flows:             make(map[Tag][]interface{}),
		lastFlushTime:     make(map[Tag]time.Time),
		flushTimers:       make(map[Tag]*time.Timer),
	}

	return s, nil
}
