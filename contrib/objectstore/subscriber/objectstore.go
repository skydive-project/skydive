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

package subscriber

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/IBM/ibm-cos-sdk-go/aws"

	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	ws "github.com/skydive-project/skydive/websocket"
)

// stream is a series of consecutive persisted objects
type stream struct {
	// ID holds the time when the stream was created
	ID time.Time
	// SeqNumber holds the next available sequence number of this stream
	SeqNumber int
}

// Storage allows writing flows to an object storage service
type Storage struct {
	bucket            string
	objectPrefix      string
	currentStream     map[tag]stream
	maxFlowsPerObject int
	maxFlowArraySize  int
	maxStreamDuration time.Duration
	maxObjectDuration time.Duration
	client            objectStoreClient
	flowTransformer   flowTransformer
	flowClassifier    flowClassifier
	excludedTags      map[tag]bool
	flows             map[tag][]interface{}
	flowsMutex        sync.Mutex
	lastFlushTime     map[tag]time.Time
	flushTimers       map[tag]*time.Timer
}

// OnStructMessage is triggered when WS server sends us a message.
func (s *Storage) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
	switch msg.Type {
	case "store":
		var flows []*flow.Flow
		if err := json.Unmarshal(msg.Obj, &flows); err != nil {
			logging.GetLogger().Error("Failed to unmarshal flows: ", err)
			return
		}

		s.StoreFlows(flows)

	default:
		logging.GetLogger().Error("Unknown message type: ", msg.Type)
	}
}

// ListObjects lists all stored objects
func (s *Storage) ListObjects() ([]*string, error) {
	objectKeys, err := s.client.ListObjects(s.bucket, s.objectPrefix)
	if err != nil {
		logging.GetLogger().Error("Failed to list objects: ", err)
		return nil, err
	}

	return objectKeys, nil
}

// ReadObjectFlows reads flows from object
func (s *Storage) ReadObjectFlows(objectKey *string, objectFlows interface{}) error {
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
func (s *Storage) DeleteObject(objectKey *string) error {
	if err := s.client.DeleteObject(s.bucket, *objectKey); err != nil {
		logging.GetLogger().Error("Failed to delete object: ", err)
		return err
	}
	return nil
}

// StoreFlows store flows in memory, before being written to the object store
func (s *Storage) StoreFlows(flows []*flow.Flow) error {
	endTime := time.Now()

	s.flowsMutex.Lock()
	defer s.flowsMutex.Unlock()

	// transform flows and save to in-memory arrays, based on tag
	if flows != nil {
		for _, fl := range flows {
			flowTag := s.flowClassifier.GetFlowTag(fl)
			if _, ok := s.excludedTags[flowTag]; ok {
				continue
			}

			var transformedFlow interface{}
			if s.flowTransformer != nil {
				transformedFlow = s.flowTransformer.Transform(fl)
			} else {
				transformedFlow = fl
			}
			if transformedFlow != nil {
				s.flows[flowTag] = append(s.flows[flowTag], transformedFlow)
			}
		}
	}

	// check which flows needs to be flushed to the object store
	flushedTags := make(map[tag]bool)
	tagsToTimerReset := make(map[tag]bool)
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

func (s *Storage) flushFlowsToObject(t tag, endTime time.Time) error {
	flows := s.flows[t]
	if len(flows) == 0 {
		return nil
	}

	if len(flows) > s.maxFlowsPerObject {
		flows = flows[:s.maxFlowsPerObject]
	}

	flowsBytes, err := json.Marshal(flows)
	if err != nil {
		logging.GetLogger().Error("Error encoding flows: ", err)
		return err
	}

	startTime := s.lastFlushTime[t]
	startTimeString := strconv.FormatInt(startTime.UTC().UnixNano()/int64(time.Millisecond), 10)
	endTimeString := strconv.FormatInt(endTime.UTC().UnixNano()/int64(time.Millisecond), 10)

	metadata := map[string]*string{
		"first-timestamp": aws.String(startTimeString),
		"last-timestamp":  aws.String(endTimeString),
		"num-records":     aws.String(strconv.Itoa(len(flows))),
	}

	// gzip
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(flowsBytes)
	w.Close()

	currentStream := s.currentStream[t]
	if endTime.Sub(currentStream.ID) >= s.maxStreamDuration {
		currentStream = stream{ID: endTime}
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

// newStorage returns a new storage interface for storing flows to object store
func newStorage(client objectStoreClient, bucket, objectPrefix string, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream int, flowTransformer flowTransformer, flowClassifier flowClassifier, excludedTags []string) *Storage {
	s := &Storage{
		bucket:            bucket,
		objectPrefix:      objectPrefix,
		maxFlowsPerObject: maxFlowsPerObject,
		maxFlowArraySize:  maxFlowArraySize,
		maxObjectDuration: time.Second * time.Duration(maxSecondsPerObject),
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		client:            client,
		flowTransformer:   flowTransformer,
		flowClassifier:    flowClassifier,
		excludedTags:      make(map[tag]bool),
		currentStream:     make(map[tag]stream),
		flows:             make(map[tag][]interface{}),
		lastFlushTime:     make(map[tag]time.Time),
		flushTimers:       make(map[tag]*time.Timer),
	}

	for _, t := range excludedTags {
		s.excludedTags[tag(t)] = true
	}

	return s
}
