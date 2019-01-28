package subscriber

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/skydive-project/skydive/contrib/objectstore/subscriber/client"
	"github.com/skydive-project/skydive/contrib/objectstore/subscriber/flowtransformer"
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
	currentStream     stream
	maxStreamDuration time.Duration
	objectStoreClient client.Client
	flowTransformer   flowtransformer.FlowTransformer
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

// StoreFlows writes a set of flows to the object storage service
func (s *Subscriber) StoreFlows(flows []*flow.Flow) error {
	if len(flows) == 0 {
		return nil
	}

	var jsonMarshalInput interface{}
	if s.flowTransformer == nil {
		jsonMarshalInput = flows
	} else {
		transformedFlows := make([]interface{}, 0, len(flows))
		for _, f := range flows {
			transformedFlow := s.flowTransformer.Transform(f)
			if transformedFlow != nil {
				transformedFlows = append(transformedFlows, transformedFlow)
			}
		}
		jsonMarshalInput = transformedFlows
	}

	flowsBytes, err := json.Marshal(jsonMarshalInput)
	if err != nil {
		logging.GetLogger().Error("Error encoding flows: ", err)
		return err
	}

	var firstTime int64 = math.MaxInt64
	var lastTime int64
	for _, fl := range flows {
		if fl.Last < firstTime {
			firstTime = fl.Last
		}
		if fl.Last > lastTime {
			lastTime = fl.Last
		}
	}
	metadata := map[string]*string{
		"first-timestamp": aws.String(strconv.FormatInt(firstTime, 10)),
		"last-timestamp":  aws.String(strconv.FormatInt(lastTime, 10)),
		"num-records":     aws.String(strconv.Itoa(len(flows))),
	}

	// gzip
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	w.Write(flowsBytes)
	w.Close()

	currentStream := s.currentStream
	if time.Since(currentStream.ID) >= s.maxStreamDuration {
		currentStream = stream{ID: time.Now()}
	}

	objectKey := strings.Join([]string{s.objectPrefix, currentStream.ID.UTC().Format("20060102T150405Z"), fmt.Sprintf("%08d.gz", currentStream.SeqNumber)}, "/")
	err = s.objectStoreClient.WriteObject(s.bucket, objectKey, string(b.Bytes()), "application/json", "gzip", metadata)

	if err != nil {
		logging.GetLogger().Error("Failed to write object: ", err)
		return err
	}

	currentStream.SeqNumber++
	s.currentStream = currentStream

	return nil
}

// New returns a new flows subscriber writing to an object storage service
func New(endpoint, region, bucket, accessKey, secretKey, objectPrefix string, maxSecondsPerStream int, flowTransformer flowtransformer.FlowTransformer) *Subscriber {
	objectStoreClient := client.New(endpoint, region, accessKey, secretKey)
	s := &Subscriber{
		bucket:            bucket,
		objectPrefix:      objectPrefix,
		maxStreamDuration: time.Second * time.Duration(maxSecondsPerStream),
		objectStoreClient: objectStoreClient,
		flowTransformer:   flowTransformer,
	}
	return s
}
