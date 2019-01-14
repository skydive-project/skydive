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
	"testing"
	"time"

	"github.com/skydive-project/skydive/flow"
)

// FakeClient is a mock Object Storage client
type FakeClient struct {
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
func (c *FakeClient) WriteObject(bucket, objectKey, data, contentType, contentEncoding string, metadata map[string]*string) error {
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
func (c *FakeClient) ReadObject(bucket, objectKey string) ([]byte, error) {
	return nil, nil
}

// ListObjects lists objects withing a bucket
func (c *FakeClient) ListObjects(bucket, prefix string) ([]*string, error) {
	return nil, nil
}

func generateFlowArray(count int) []*flow.Flow {
	flows := make([]*flow.Flow, count)
	for i := 0; i < count; i++ {
		fl := &flow.Flow{}
		flows[i] = fl
	}

	return flows
}

func newTestSubscriber() (*Subscriber, *FakeClient) {
	fakeClient := &FakeClient{}
	ds := &Subscriber{
		maxStreamDuration: time.Duration(24 * time.Hour),
		bucket:            "myBucket",
		objectPrefix:      "myPrefix",
		objectStoreClient: fakeClient,
	}

	return ds, fakeClient
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

func Test_StoreFlows_Positive(t *testing.T) {
	c, fakeClient := newTestSubscriber()
	flows := generateFlowArray(10)
	for i := 0; i < 10; i++ {
		flows[i].Last = int64(i)
	}

	assertEqual(t, nil, c.StoreFlows(flows))
	assertEqual(t, 1, fakeClient.WriteCounter)
	assertEqual(t, 1, c.currentStream.SeqNumber)

	// gzip decompress
	r, err := gzip.NewReader(bytes.NewBuffer([]byte(fakeClient.LastData)))
	defer r.Close()
	if err != nil {
		t.Fatal("Error in gzip decompress: ", err)
	}
	uncompressedData, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal("Error in gzip reading: ", err)
	}

	var decodedFlows []*flow.Flow
	if err := json.Unmarshal(uncompressedData, &decodedFlows); err != nil {
		t.Fatal("JSON parsing failed: ", err)
	}

	assertEqual(t, len(flows), len(decodedFlows))
	assertEqual(t, "0", *fakeClient.LastMetadata["first-timestamp"])
	assertEqual(t, "9", *fakeClient.LastMetadata["last-timestamp"])
	assertEqual(t, strconv.Itoa(len(flows)), *fakeClient.LastMetadata["num-records"])

	assertEqual(t, "application/json", fakeClient.LastContentType)
	assertEqual(t, "gzip", fakeClient.LastContentEncoding)
}

func Test_StoreFlows_maxStreamDuration(t *testing.T) {
	c, _ := newTestSubscriber()
	c.maxStreamDuration = time.Second

	c.StoreFlows(generateFlowArray(1))
	assertEqual(t, 1, c.currentStream.SeqNumber)
	streamID := c.currentStream.ID
	c.StoreFlows(generateFlowArray(1))
	assertEqual(t, 2, c.currentStream.SeqNumber)
	assertEqual(t, streamID, c.currentStream.ID)

	time.Sleep(time.Second)

	c.StoreFlows(generateFlowArray(1))
	assertEqual(t, 1, c.currentStream.SeqNumber)
	assertNotEqual(t, streamID, c.currentStream.ID)
}

func Test_StoreFlows_WriteObjectError(t *testing.T) {
	c, fakeClient := newTestSubscriber()
	myError := errors.New("my error")
	fakeClient.WriteError = myError

	assertEqual(t, myError, c.StoreFlows(generateFlowArray(1)))
}
