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

// Subscriber describes a Flows subscriber writing to an object storage service
type Subscriber struct {
	bucket            string
	objectPrefix      string
	currentStream     map[Tag]stream
	maxFlowsPerObject int
	maxFlowArraySize  int
	maxStreamDuration time.Duration
	maxObjectDuration time.Duration
	objectStoreClient Client
	flowTransformer   FlowTransformer
	flowClassifier    FlowClassifier
	flows             map[Tag][]interface{}
	flowsMutex        sync.Mutex
	lastFlushTime     map[Tag]time.Time
	flushTimers       map[Tag]*time.Timer
}

// OnStructMessage is triggered when WS server sends us a message.
func (s *Subscriber) OnStructMessage(c ws.Speaker, msg *ws.StructMessage) {
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

// StoreFlows store flows in memory, before being written to the object store
func (s *Subscriber) StoreFlows(flows []*flow.Flow) error {
	endTime := time.Now()

	s.flowsMutex.Lock()
	defer s.flowsMutex.Unlock()

	// transform flows and save to in-memory arrays, based on tag
	if flows != nil {
		for _, fl := range flows {
			var transformedFlow interface{}
			if s.flowTransformer != nil {
				transformedFlow = s.flowTransformer.Transform(fl)
			} else {
				transformedFlow = fl
			}
			if transformedFlow != nil {
				flowTag := s.flowClassifier.GetFlowTag(fl)
				s.flows[flowTag] = append(s.flows[flowTag], transformedFlow)
			}
		}
	}

	// check which flows needs to be flushed to the object store
	flushedTags := make(map[Tag]bool)
	tagsToTimerReset := make(map[Tag]bool)
	for tag := range s.flows {
		if _, ok := s.lastFlushTime[tag]; !ok {
			s.lastFlushTime[tag] = endTime
			tagsToTimerReset[tag] = true
		}
		for {
			flowsLength := len(s.flows[tag])
			if flowsLength == 0 {
				break
			}
			if flowsLength < s.maxFlowsPerObject && time.Since(s.lastFlushTime[tag]) < s.maxObjectDuration {
				break
			}

			if s.flushFlowsToObject(tag, endTime) != nil {
				break
			}

			flushedTags[tag] = true
			tagsToTimerReset[tag] = true
		}

		// check if need to trim flow array
		overflowCount := len(s.flows["tag"]) - s.maxFlowArraySize
		if overflowCount > 0 {
			logging.GetLogger().Warningf("Flow array for tag '%s' overflowed maximum size. Discarding %d flows", string(tag), overflowCount)
			s.flows["tag"] = s.flows["tag"][overflowCount:]
		}
	}

	for tag := range flushedTags {
		// copy slice to free up memory of already flushed flows
		newArray := make([]interface{}, 0, len(s.flows[tag]))
		copy(newArray, s.flows[tag])
		s.flows[tag] = newArray
	}

	for tag := range tagsToTimerReset {
		// stop old flush timers
		oldTimer, ok := s.flushTimers[tag]
		if ok {
			oldTimer.Stop()
		}
	}

	// set a timer for next flush
	for tag := range tagsToTimerReset {
		s.flushTimers[tag] = time.AfterFunc(s.maxObjectDuration-time.Since(endTime), func() { s.StoreFlows(nil) })

		// a single timer will cover all of the flushed tags
		break
	}

	return nil
}

func (s *Subscriber) flushFlowsToObject(tag Tag, endTime time.Time) error {
	flows := s.flows[tag]
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

	startTime := s.lastFlushTime[tag]
	startTimeString := strconv.FormatInt(startTime.UTC().UnixNano()/int64(time.Millisecond), 10)
	endTimeString := strconv.FormatInt(endTime.UTC().UnixNano()/int64(time.Millisecond), 10)

	metadata := map[string]*string{
		"start-timestamp": aws.String(startTimeString),
		"end-timestamp":   aws.String(endTimeString),
		"num-records":     aws.String(strconv.Itoa(len(flows))),
	}

	// gzip
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(flowsBytes)
	w.Close()

	currentStream := s.currentStream[tag]
	if endTime.Sub(currentStream.ID) >= s.maxStreamDuration {
		currentStream = stream{ID: endTime}
	}

	objectKey := strings.Join([]string{s.objectPrefix, string(tag), currentStream.ID.UTC().Format("20060102T150405Z"), fmt.Sprintf("%08d.gz", currentStream.SeqNumber)}, "/")
	err = s.objectStoreClient.WriteObject(s.bucket, objectKey, string(b.Bytes()), "application/json", "gzip", metadata)

	if err != nil {
		logging.GetLogger().Error("Failed to write object: ", err)
		return err
	}

	// flush was successful, update state
	s.lastFlushTime[tag] = endTime
	currentStream.SeqNumber++
	s.currentStream[tag] = currentStream
	s.flows[tag] = s.flows[tag][len(flows):]

	return nil
}

// New returns a new flows subscriber writing to an object storage service
func New(objectStoreClient Client, bucket, objectPrefix string, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream int, flowTransformer FlowTransformer, flowClassifier FlowClassifier) *Subscriber {
	s := &Subscriber{
		bucket:            bucket,
		objectPrefix:      objectPrefix,
		maxFlowsPerObject: maxFlowsPerObject,
		maxFlowArraySize:  maxFlowArraySize,
		maxObjectDuration: time.Second * time.Duration(maxSecondsPerObject),
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		objectStoreClient: objectStoreClient,
		flowTransformer:   flowTransformer,
		flowClassifier:    flowClassifier,
		currentStream:     make(map[Tag]stream),
		flows:             make(map[Tag][]interface{}),
		lastFlushTime:     make(map[Tag]time.Time),
		flushTimers:       make(map[Tag]*time.Timer),
	}
	return s
}
