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
	"errors"
	"fmt"
	"io/ioutil"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/skydive-project/skydive/flow"
)

const (
	bucket              = "myBucket"
	objectPrefix        = "myPrefix"
	maxFlowsPerObject   = 6000
	maxSecondsPerObject = 60
	maxSecondsPerStream = 86400
	maxFlowArraySize    = 100000
)

var (
	testFlowClassifier  = &fakeFlowClassifier{}
	testFlowTransformer flowTransformer
)

type fakeClient struct {
	// WriteError holds the error that should be returned on WriteObject
	WriteError error
	// WriteCounter holds the number of times that WriteObject was called
	WriteCounter int
	// LastBucket holds the bucket argument used on the last call to WriteObject
	LastBucket string
	// LastObjectKey holds the objectKey argument used on the last call to WriteObject
	LastObjectKey string
	// LastData holds the data argument used on the last call to WriteObject
	LastData string
	// LastContentType holds the contentType argument used on the last call to WriteObject
	LastContentType string
	// LastContentEncoding holds the contentEncoding argument used on the last call to WriteObject
	LastContentEncoding string
	// LastMetadata holds the metadata argument used on the last call to WriteObject
	LastMetadata map[string]*string
}

// WriteObject stores a single object
func (c *fakeClient) WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error {
	c.WriteCounter++
	c.LastBucket = bucket
	c.LastObjectKey = objectKey
	c.LastData = data
	c.LastContentType = contentType
	c.LastContentEncoding = contentEncoding
	c.LastMetadata = metadata

	return c.WriteError
}

// ReadObject reads a single object
func (c *fakeClient) ReadObject(bucket, objectKey string) ([]byte, error) {
	return nil, nil
}

// ReadObjectMetadata returns an object metadata
func (c *fakeClient) ReadObjectMetadata(bucket, objectKey string) (map[string]*string, error) {
	return nil, nil
}

// DeleteObject deletes a single object
func (c *fakeClient) DeleteObject(bucket, objectKey string) error {
	return nil
}

// ListObjects lists objects within a bucket
func (c *fakeClient) ListObjects(bucket, prefix string) ([]*string, error) {
	return nil, nil
}

// fakeFlowClassifier is a mock flow classifier
type fakeFlowClassifier struct {
}

// GetFlowTag tag flows according to their UUID
func (fc *fakeFlowClassifier) GetFlowTag(fl *flow.Flow) tag {
	return tag(fl.UUID)
}

func generateFlowArray(count int, tag string) []*flow.Flow {
	flows := make([]*flow.Flow, count)
	for i := 0; i < count; i++ {
		fl := &flow.Flow{}
		fl.UUID = tag
		flows[i] = fl
	}

	return flows
}

func newTestStorage() (*Storage, *fakeClient) {
	client := &fakeClient{}
	return newStorage(client, bucket, objectPrefix, maxFlowArraySize, maxFlowsPerObject, maxSecondsPerObject, maxSecondsPerStream, testFlowTransformer, testFlowClassifier, []string{}), client
}

func assertEqual(t *testing.T, expected, actual interface{}) {
	if expected != actual {
		msg := "Equal assertion failed"
		_, file, no, ok := runtime.Caller(1)
		if ok {
			msg += fmt.Sprintf(" on %s:%d", file, no)
		}
		t.Fatalf("%s: (expected: %v, actual: %v)", msg, expected, actual)
	}
}

func assertNotEqual(t *testing.T, notExpected, actual interface{}) {
	if notExpected == actual {
		msg := "NotEqual assertion failed"
		_, file, no, ok := runtime.Caller(1)
		if ok {
			msg += fmt.Sprintf(" on %s:%d", file, no)
		}
		t.Fatalf("%s: (notExpected: %v, actual: %v)", msg, notExpected, actual)
	}
}

func Test_flushFlowsToObject_NoFlows(t *testing.T) {
	s, client := newTestStorage()
	assertEqual(t, nil, s.flushFlowsToObject("tag", time.Now()))
	assertEqual(t, 0, client.WriteCounter)
}

func Test_flushFlowsToObject_MarshalError(t *testing.T) {
	s, client := newTestStorage()
	s.flows["tag"] = []interface{}{make(chan int)}
	err := s.flushFlowsToObject("tag", time.Now())
	assertNotEqual(t, nil, err)
	assertEqual(t, 0, client.WriteCounter)
	assertEqual(t, 1, len(s.flows["tag"]))
	_, ok := err.(*json.UnsupportedTypeError)
	assertEqual(t, true, ok)
}

func Test_flushFlowsToObject_maxStreamDuration(t *testing.T) {
	s, _ := newTestStorage()
	s.maxStreamDuration = time.Second * time.Duration(2)

	s.flows["tag"] = make([]interface{}, 1)
	assertEqual(t, nil, s.flushFlowsToObject("tag", time.Unix(60, 0)))
	assertEqual(t, 1, s.currentStream["tag"].SeqNumber)
	assertEqual(t, time.Unix(60, 0), s.currentStream["tag"].ID)

	s.flows["tag"] = make([]interface{}, 1)
	assertEqual(t, nil, s.flushFlowsToObject("tag", time.Unix(61, 0)))
	assertEqual(t, 2, s.currentStream["tag"].SeqNumber)
	assertEqual(t, time.Unix(60, 0), s.currentStream["tag"].ID)

	time.Sleep(time.Second)

	s.flows["tag"] = make([]interface{}, 1)
	assertEqual(t, nil, s.flushFlowsToObject("tag", time.Unix(62, 0)))
	assertEqual(t, 1, s.currentStream["tag"].SeqNumber)
	assertEqual(t, time.Unix(62, 0), s.currentStream["tag"].ID)
}

func Test_flushFlowsToObject_WriteObjectError(t *testing.T) {
	s, client := newTestStorage()
	client.WriteError = errors.New("my error")

	s.flows["tag"] = make([]interface{}, 1)
	assertEqual(t, client.WriteError, s.flushFlowsToObject("tag", time.Now()))
	assertEqual(t, 1, len(s.flows["tag"]))
}

func Test_flushFlowsToObject_Positive(t *testing.T) {
	s, client := newTestStorage()
	s.flows["tag"] = make([]interface{}, 10)
	s.lastFlushTime["tag"] = time.Unix(1, 0)
	assertEqual(t, nil, s.flushFlowsToObject("tag", time.Unix(2, 0)))
	assertEqual(t, 0, len(s.flows["tag"]))
	assertEqual(t, 1, s.currentStream["tag"].SeqNumber)

	assertEqual(t, 1, client.WriteCounter)
	assertEqual(t, bucket, client.LastBucket)
	assertEqual(t, "gzip", client.LastContentEncoding)
	assertEqual(t, "application/json", client.LastContentType)
	assertEqual(t, true, strings.Contains(client.LastObjectKey, objectPrefix))
	assertEqual(t, true, strings.Contains(client.LastObjectKey, "tag"))
	assertEqual(t, true, strings.Contains(client.LastObjectKey, s.currentStream["tag"].ID.UTC().Format("20060102T150405Z")))
	assertEqual(t, true, strings.Contains(client.LastObjectKey, "00000000.gz"))

	// gzip decompress
	r, err := gzip.NewReader(bytes.NewBuffer([]byte(client.LastData)))
	defer r.Close()
	if err != nil {
		t.Fatal("Error in gzip decompress: ", err)
	}
	uncompressedData, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("Error in gzip reading: ", err)
	}

	var decodedFlows []interface{}
	if err := json.Unmarshal(uncompressedData, &decodedFlows); err != nil {
		t.Fatal("JSON parsing failed: ", err)
	}

	assertEqual(t, 10, len(decodedFlows))

	assertEqual(t, "1000", *client.LastMetadata["first-timestamp"])
	assertEqual(t, "2000", *client.LastMetadata["last-timestamp"])
	assertEqual(t, strconv.FormatInt(s.lastFlushTime["tag"].UTC().UnixNano()/int64(time.Millisecond), 10), *client.LastMetadata["last-timestamp"])
	assertEqual(t, "10", *client.LastMetadata["num-records"])
}

func Test_StoreFlows_MaxFlowsPerObject(t *testing.T) {
	s, client := newTestStorage()
	s.maxFlowsPerObject = 10
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(5, "tag")))
	assertEqual(t, 0, client.WriteCounter)
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(5, "tag")))
	assertEqual(t, 1, client.WriteCounter)
	client.WriteCounter = 0
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(35, "tag")))
	assertEqual(t, 3, client.WriteCounter)
}

func Test_StoreFlows_MaxObjectDuration(t *testing.T) {
	s, client := newTestStorage()
	s.maxObjectDuration = time.Second
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(1, "tag")))
	assertEqual(t, 0, client.WriteCounter)
	time.Sleep(time.Second / 2)
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(1, "tag")))
	s.flowsMutex.Lock()
	assertEqual(t, 0, client.WriteCounter)
	s.flowsMutex.Unlock()

	// Wait half a second + little (to avoid racing)
	time.Sleep(time.Second)
	s.flowsMutex.Lock()
	assertEqual(t, 1, client.WriteCounter)
	s.flowsMutex.Unlock()
}

func Test_StoreFlows_WriteError(t *testing.T) {
	s, client := newTestStorage()
	s.maxFlowsPerObject = 1
	client.WriteError = errors.New("my error")
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(1, "tag")))
	assertEqual(t, 1, len(s.flows["tag"]))
}

func Test_StoreFlows_maxFlowArraySize(t *testing.T) {
	s, client := newTestStorage()
	s.maxFlowArraySize = 10
	client.WriteError = errors.New("my error")
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(11, "tag")))
	assertEqual(t, 10, len(s.flows["tag"]))
}

func Test_StoreFlows_ExcludedTags(t *testing.T) {
	s, client := newTestStorage()
	s.excludedTags["tag"] = true
	assertEqual(t, nil, s.StoreFlows(generateFlowArray(1, "tag")))
	assertEqual(t, 0, client.WriteCounter)
	assertEqual(t, 0, len(s.flows["tag"]))
}
