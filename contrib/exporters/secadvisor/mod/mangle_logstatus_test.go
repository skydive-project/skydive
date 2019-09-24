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

package mod

import (
	"bytes"
	"testing"

	"github.com/spf13/viper"

	awsflowlogs "github.com/skydive-project/skydive/contrib/exporters/awsflowlogs/mod"
	"github.com/skydive-project/skydive/contrib/exporters/core"
	"github.com/skydive-project/skydive/flow"
)

var logStatusConfig = []byte(`---
pipeline:
  mangle:
    type: logstatus
`)

func ConfigFromJSON(t *testing.T, content []byte) *viper.Viper {
	cfg := viper.New()
	cfg.SetConfigType("yaml")
	r := bytes.NewBuffer(content)
	if err := cfg.ReadConfig(r); err != nil {
		t.Fatalf("Failed to read config: %s", err)
	}
	return cfg
}

func getMangleLogStatus(t *testing.T) core.Mangler {
	cfg := ConfigFromJSON(t, logStatusConfig)
	mangler, err := NewMangleLogStatus(cfg)
	if err != nil {
		t.Fatalf("Mangle creation returned unexpected error: %s", err)
	}
	return mangler.(core.Mangler)
}

func filterByLogStatus(in []interface{}, logStatus awsflowlogs.LogStatus) (out []interface{}) {
	for _, flow := range in {
		flow := flow.(*SecurityAdvisorFlow)
		if flow.LogStatus == string(logStatus) {
			out = append(out, flow)
		}
	}
	return
}

func TestMangleLogStatusOK(t *testing.T) {
	mangler := getMangleLogStatus(t)

	in := []interface{}{
		&SecurityAdvisorFlow{
			UUID: "A",
		},
	}

	out := mangler.Mangle(in)
	out = filterByLogStatus(out, awsflowlogs.LogStatusOk)

	if len(out) != 1 {
		t.Fatalf("Expected 1 flow entry but got %d", len(out))
	}

	flow := out[0].(*SecurityAdvisorFlow)
	if flow.UUID != "A" {
		t.Fatalf("Expected flow.UUID 'A' but got: '%s'", flow.UUID)
	}
}

func TestMangleLogStatusNoData(t *testing.T) {
	mangler := getMangleLogStatus(t)

	in1 := []interface{}{
		&SecurityAdvisorFlow{
			L3TrackingID: "ok",
			LinkID:       1,
		},
		&SecurityAdvisorFlow{
			L3TrackingID: "nodata",
			LinkID:       2,
		},
	}

	in2 := []interface{}{
		&SecurityAdvisorFlow{
			L3TrackingID: "ok",
			LinkID:       1,
		},
	}

	mangler.Mangle(in1)
	out := mangler.Mangle(in2)
	out = filterByLogStatus(out, awsflowlogs.LogStatusNoData)

	if len(out) != 1 {
		t.Fatalf("Expected 1 flow entry but got: %d", len(out))
	}

	flow := out[0].(*SecurityAdvisorFlow)

	if flow.LinkID != 2 {
		t.Fatalf("Expected flow.LinkData '2' but got: '%d'", flow.LinkID)
	}
}

func TestMangleLogStatusSkipData(t *testing.T) {
	mangler := getMangleLogStatus(t)

	in1 := []interface{}{
		&SecurityAdvisorFlow{
			L3TrackingID: "1",
			UUID:         "A",
			LinkID:       1,
			Metric: &flow.FlowMetric{
				Start: 1,
			},
		},
	}

	in2 := []interface{}{
		&SecurityAdvisorFlow{
			L3TrackingID: "1",
			UUID:         "A",
			LinkID:       1,
			LastUpdateMetric: &flow.FlowMetric{
				Start: 2,
			},
		},
	}

	mangler.Mangle(in1)
	out := mangler.Mangle(in2)
	out = filterByLogStatus(out, awsflowlogs.LogStatusSkipData)

	if len(out) != 1 {
		t.Fatalf("Expected 1 flow entry but got: %d", len(out))
	}

	flow := out[0].(*SecurityAdvisorFlow)

	if flow.UUID != "A" {
		t.Fatalf("Expected flow.UUID 'A' but got: '%s'", flow.UUID)
	}
}
