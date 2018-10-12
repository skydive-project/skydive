/*
 * Copyright (C) 2017 Red Hat, Inc.
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

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/flow"
	"github.com/skydive-project/skydive/logging"
	es "github.com/skydive-project/skydive/storage/elasticsearch"
)

// sudo -E /usr/bin/snort -A cmg -c /etc/snort/snort.lua -R snort3-community-rules/snort3-community.rules -i br-gre -X 2>/dev/null | go run contrib/snort/snortSkydive.go

const snortMessageMapping = `
{
	"dynamic_templates": [
		{
			"timestamp": {
				"match": "Timestamp",
				"mapping": {
					"type": "date", "format": "epoch_millis"
				}
			}
		}
	]
}`

var snortIndex = es.Index{
	Name:    "snort",
	Type:    "snort_message",
	Mapping: snortMessageMapping,
}

const (
	timestamp = iota
	stepSnortRAW
	stepDecodeHEX
	stepEnd
)

const sepHEX = "- -   - - - - - - - - - - - -  - - - - - - - - - - - -  - - - - - - - - -\n"

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// SnortFlowEnhancer describes a snort graph enhancer
type SnortFlowEnhancer struct {
	client  *es.Client
	running atomic.Value
	quit    chan bool
}

type snortMessage struct {
	TrackingID     string
	Timestamp      time.Time
	Message        string
	Classification string
	Data           []byte
}

func flowFromSnortMessage(msg *snortMessage) *flow.Flow {
	uuids := flow.UUIDs{}
	nodeTID := ""
	gpkt := gopacket.NewPacket(msg.Data, layers.LayerTypeEthernet, gopacket.NoCopy)
	gpkt.Metadata().CaptureInfo.Timestamp = msg.Timestamp

	return flow.NewFlowFromGoPacket(gpkt, nodeTID, uuids, flow.Opts{})
}

func parseSnortTimestamp(timestamp string) time.Time {
	t := time.Now()
	var months, days, hour, min, sec, usec int
	fmt.Sscanf(timestamp, "%d/%d-%d:%d:%d.%d",
		&months, &days, &hour, &min, &sec, &usec)
	year, _, _ := t.Date()
	return time.Date(year, time.Month(months), days, hour, min, sec, usec*1000, t.Location()).UTC()
}

func (sfe *SnortFlowEnhancer) insertElasticSearch(msg *snortMessage, f *flow.Flow) error {
	if !sfe.client.Started() {
		return fmt.Errorf("Storage is not yet started")
	}

	snortMessage := map[string]interface{}{
		"TrackingID":     f.TrackingID,
		"Timestamp":      msg.Timestamp,
		"Message":        msg.Message,
		"Classification": msg.Classification,
	}

	if err := sfe.client.BulkIndex(snortIndex, "", snortMessage); err != nil {
		return fmt.Errorf("Error while indexing: %s", err.Error())
	}

	logging.GetLogger().Infof("insert flow TrackingID %s %+#v", f.TrackingID, f)
	return nil
}

func (sfe *SnortFlowEnhancer) recvSnortMessage(msg *snortMessage) {
	f := flowFromSnortMessage(msg)

	if err := sfe.insertElasticSearch(msg, f); err != nil {
		logging.GetLogger().Error(err)
	}
}

// readCMGX : read snort log with running with "-A cmg -X" options
func (sfe *SnortFlowEnhancer) parseSnortCMGX(reader *bufio.Reader) error {
	regexTimestamp := regexp.MustCompile("^(?P<timestamp>\\d\\d/\\d\\d-\\d\\d:\\d\\d:\\d\\d\\.\\d\\d\\d\\d\\d\\d) \\[.+?\\] \\[.+?\\] \\\"(?P<message>.+?)\\\" \\[.+?\\] \\[(?P<classification>.+?)\\].*\n")
	regexSnortRAW := regexp.MustCompile("^snort\\.raw\\[(?P<bytes>\\d+)\\]:\n")

	var packetBytes int
	var msg *snortMessage
	step := timestamp
	for sfe.running.Load() == true {
		str, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		if step == timestamp && regexTimestamp.MatchString(str) {
			msg = &snortMessage{}
			msg.Timestamp = parseSnortTimestamp(regexTimestamp.ReplaceAllString(str, "${timestamp}"))
			msg.Message = regexTimestamp.ReplaceAllString(str, "${message}")
			msg.Classification = regexTimestamp.ReplaceAllString(str, "${classification}")
			step = stepSnortRAW
			continue
		}
		if step == stepSnortRAW && regexSnortRAW.MatchString(str) {
			packetBytes, err = strconv.Atoi(regexSnortRAW.ReplaceAllString(str, "${bytes}"))
			if err != nil {
				return err
			}
			step = stepDecodeHEX
			continue
		}
		if step == stepDecodeHEX {
			if str == sepHEX {
				if len(msg.Data) > 0 {
					step = stepEnd
				}
				continue
			}
			v := make([]byte, 8)
			vals := strings.Split(str, "  ")
			for block := 1; block <= min(2, len(vals)-1); block++ {
				n, _ := fmt.Sscanf(vals[block], "%X %X %X %X %X %X %X %X",
					&v[0], &v[1], &v[2], &v[3], &v[4], &v[5], &v[6], &v[7])
				for i := 0; i < n; i++ {
					msg.Data = append(msg.Data, v[i])
				}
			}
		}
		if step == stepEnd {
			if packetBytes != len(msg.Data) {
				return fmt.Errorf("msg.packetBytes(%d) != len(msg.Data)(%d)", packetBytes, len(msg.Data))
			}
			sfe.recvSnortMessage(msg)
			step = timestamp
		}
	}
	return nil
}

func (sfe *SnortFlowEnhancer) start() {
	go sfe.client.Start()
	go sfe.run()
}

func (sfe *SnortFlowEnhancer) run() {
	reader := bufio.NewReader(os.Stdin)
	err := sfe.parseSnortCMGX(reader)
	if err != nil {
		logging.GetLogger().Errorf("parse error : %v", err)
		return
	}
	sfe.quit <- true
}

func (sfe *SnortFlowEnhancer) stop() {
	sfe.running.Store(false)
	os.Stdin.Close()
	sfe.client.Stop()
	<-sfe.quit
}

func newSnortFlowEnhancer() *SnortFlowEnhancer {
	sfe := &SnortFlowEnhancer{}
	sfe.quit = make(chan bool)
	sfe.running.Store(true)

	indices := []es.Index{snortIndex}

	cfg := es.NewConfig()
	client, err := es.NewClient(indices, cfg, nil)
	if err != nil {
		if err != io.EOF {
			logging.GetLogger().Errorf("elasticsearch client error : %v", err)
			return nil
		}
	}
	sfe.client = client

	return sfe
}

func main() {
	config.Set("logging.id", "SnortToSkydive")
	sfe := newSnortFlowEnhancer()
	if sfe == nil {
		return
	}
	sfe.start()
	logging.GetLogger().Info("Snort to Skydive started")

	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch

	sfe.stop()
	logging.GetLogger().Info("Snort to Skydive stopped")
}
