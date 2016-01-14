/*
 * Copyright (C) 2015 Red Hat, Inc.
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

package graph

import (
	"encoding/json"
	"fmt"
	"go/token"
	"os"
	"time"

	"github.com/nu7hatch/gouuid"
	eval "github.com/sbinet/go-eval"

	"github.com/redhat-cip/skydive/logging"
)

type Alert struct {
	Client *AsyncClient
	Graph  *Graph
	alerts []AlertTest
}

type AlertType int

const (
	FIXED AlertType = 1 + iota
	THRESHOLD
)

type AlertTest struct {
	UUID        *uuid.UUID
	Name        string
	Description string
	CreateTime  time.Time

	Select string
	Test   string

	Type   AlertType
	Action string
	Count  int
}

type AlertMessage struct {
	UUID       uuid.UUID
	Type       AlertType
	Timestamp  time.Time
	Count      int
	Reason     string
	ReasonData string
}

func (d *AlertMessage) String() string {
	j, _ := json.Marshal(d)
	return string(j)
}

func UnmarshalAlertMessage(b []byte) (AlertMessage, error) {
	msg := AlertMessage{}

	err := json.Unmarshal(b, &msg)
	if err != nil {
		return msg, err
	}

	return msg, nil
}

func (c *Alert) Register(name string, desc string, selection string, test string, action string) {
	id, _ := uuid.NewV4()
	a := AlertTest{
		UUID:        id,
		Name:        name,
		Description: desc,
		CreateTime:  time.Now(),

		Select: selection,
		Test:   test,
		Type:   FIXED,
		Action: action,
		Count:  0,
	}
	c.alerts = append(c.alerts, a)
}

func (c *Alert) EvalNodes() {
	for _, a := range c.alerts {
		nodes := c.Graph.LookupNodesFromKey(a.Select)
		for _, n := range nodes {
			w := eval.NewWorld()
			defConst := func(name string, val interface{}) {
				t, v := toTypeValue(val)
				w.DefineConst(name, t, v)
			}
			for k, v := range n.Metadatas {
				defConst(k, v)
			}
			fs := token.NewFileSet()
			toEval := "(" + a.Test + ") == true"
			expr, err := w.Compile(fs, toEval)
			if err != nil {
				logging.GetLogger().Error("Can't compile expression : " + toEval)
				continue
			}
			ret, err := expr.Run()
			if err != nil {
				logging.GetLogger().Error("Can't evaluate expression : " + toEval)
				continue
			}

			if ret.String() == "true" {
				a.Count++
				msg := AlertMessage{
					UUID:       *a.UUID,
					Type:       FIXED,
					Timestamp:  time.Now(),
					Count:      a.Count,
					Reason:     a.Action,
					ReasonData: toEval + " " + n.String(),
				}
				c.Client.SendAlertMessage(msg)
			}
		}
	}
}

func (c *Alert) triggerResync() {
	logging.GetLogger().Info("Start a resync of the graph")

	hostname, err := os.Hostname()
	if err != nil {
		logging.GetLogger().Error("Unable to retrieve the hostname: %s", err.Error())
		return
	}

	c.Graph.Lock()
	defer c.Graph.Unlock()

	// request for deletion of everything belonging to host node
	root := c.Graph.GetNode(Identifier(hostname))
	if root == nil {
		return
	}
}

func (c *Alert) OnConnected() {
	c.triggerResync()
}

func (c *Alert) OnDisconnected() {
}

func (c *Alert) OnNodeUpdated(n *Node) {
	c.EvalNodes()
}

func (c *Alert) OnNodeAdded(n *Node) {
	c.EvalNodes()
}

func (c *Alert) OnNodeDeleted(n *Node) {
}

func (c *Alert) OnEdgeUpdated(e *Edge) {
}

func (c *Alert) OnEdgeAdded(e *Edge) {
}

func (c *Alert) OnEdgeDeleted(e *Edge) {
}

func NewAlert(c *AsyncClient, g *Graph) *Alert {
	f := &Alert{
		Client: c,
		Graph:  g,
	}

	g.AddEventListener(f)
	c.AddListener(f)

	return f
}

/*
 * go-eval helpers
 */

type boolV bool

func (v *boolV) String() string                      { return fmt.Sprint(*v) }
func (v *boolV) Assign(t *eval.Thread, o eval.Value) { *v = boolV(o.(eval.BoolValue).Get(t)) }
func (v *boolV) Get(*eval.Thread) bool               { return bool(*v) }
func (v *boolV) Set(t *eval.Thread, x bool)          { *v = boolV(x) }

type uint8V uint8

func (v *uint8V) String() string                      { return fmt.Sprint(*v) }
func (v *uint8V) Assign(t *eval.Thread, o eval.Value) { *v = uint8V(o.(eval.UintValue).Get(t)) }
func (v *uint8V) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uint8V) Set(t *eval.Thread, x uint64)        { *v = uint8V(x) }

type uint16V uint16

func (v *uint16V) String() string                      { return fmt.Sprint(*v) }
func (v *uint16V) Assign(t *eval.Thread, o eval.Value) { *v = uint16V(o.(eval.UintValue).Get(t)) }
func (v *uint16V) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uint16V) Set(t *eval.Thread, x uint64)        { *v = uint16V(x) }

type uint32V uint32

func (v *uint32V) String() string                      { return fmt.Sprint(*v) }
func (v *uint32V) Assign(t *eval.Thread, o eval.Value) { *v = uint32V(o.(eval.UintValue).Get(t)) }
func (v *uint32V) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uint32V) Set(t *eval.Thread, x uint64)        { *v = uint32V(x) }

type uint64V uint64

func (v *uint64V) String() string                      { return fmt.Sprint(*v) }
func (v *uint64V) Assign(t *eval.Thread, o eval.Value) { *v = uint64V(o.(eval.UintValue).Get(t)) }
func (v *uint64V) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uint64V) Set(t *eval.Thread, x uint64)        { *v = uint64V(x) }

type uintV uint

func (v *uintV) String() string                      { return fmt.Sprint(*v) }
func (v *uintV) Assign(t *eval.Thread, o eval.Value) { *v = uintV(o.(eval.UintValue).Get(t)) }
func (v *uintV) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uintV) Set(t *eval.Thread, x uint64)        { *v = uintV(x) }

type uintptrV uintptr

func (v *uintptrV) String() string                      { return fmt.Sprint(*v) }
func (v *uintptrV) Assign(t *eval.Thread, o eval.Value) { *v = uintptrV(o.(eval.UintValue).Get(t)) }
func (v *uintptrV) Get(*eval.Thread) uint64             { return uint64(*v) }
func (v *uintptrV) Set(t *eval.Thread, x uint64)        { *v = uintptrV(x) }

/*
 * Int
 */

type int8V int8

func (v *int8V) String() string                      { return fmt.Sprint(*v) }
func (v *int8V) Assign(t *eval.Thread, o eval.Value) { *v = int8V(o.(eval.IntValue).Get(t)) }
func (v *int8V) Get(*eval.Thread) int64              { return int64(*v) }
func (v *int8V) Set(t *eval.Thread, x int64)         { *v = int8V(x) }

type int16V int16

func (v *int16V) String() string                      { return fmt.Sprint(*v) }
func (v *int16V) Assign(t *eval.Thread, o eval.Value) { *v = int16V(o.(eval.IntValue).Get(t)) }
func (v *int16V) Get(*eval.Thread) int64              { return int64(*v) }
func (v *int16V) Set(t *eval.Thread, x int64)         { *v = int16V(x) }

type int32V int32

func (v *int32V) String() string                      { return fmt.Sprint(*v) }
func (v *int32V) Assign(t *eval.Thread, o eval.Value) { *v = int32V(o.(eval.IntValue).Get(t)) }
func (v *int32V) Get(*eval.Thread) int64              { return int64(*v) }
func (v *int32V) Set(t *eval.Thread, x int64)         { *v = int32V(x) }

type int64V int64

func (v *int64V) String() string                      { return fmt.Sprint(*v) }
func (v *int64V) Assign(t *eval.Thread, o eval.Value) { *v = int64V(o.(eval.IntValue).Get(t)) }
func (v *int64V) Get(*eval.Thread) int64              { return int64(*v) }
func (v *int64V) Set(t *eval.Thread, x int64)         { *v = int64V(x) }

type intV int

func (v *intV) String() string                      { return fmt.Sprint(*v) }
func (v *intV) Assign(t *eval.Thread, o eval.Value) { *v = intV(o.(eval.IntValue).Get(t)) }
func (v *intV) Get(*eval.Thread) int64              { return int64(*v) }
func (v *intV) Set(t *eval.Thread, x int64)         { *v = intV(x) }

/*
 * Float
 */

type float32V float32

func (v *float32V) String() string                      { return fmt.Sprint(*v) }
func (v *float32V) Assign(t *eval.Thread, o eval.Value) { *v = float32V(o.(eval.FloatValue).Get(t)) }
func (v *float32V) Get(*eval.Thread) float64            { return float64(*v) }
func (v *float32V) Set(t *eval.Thread, x float64)       { *v = float32V(x) }

type float64V float64

func (v *float64V) String() string                      { return fmt.Sprint(*v) }
func (v *float64V) Assign(t *eval.Thread, o eval.Value) { *v = float64V(o.(eval.FloatValue).Get(t)) }
func (v *float64V) Get(*eval.Thread) float64            { return float64(*v) }
func (v *float64V) Set(t *eval.Thread, x float64)       { *v = float64V(x) }

/*
 * String
 */

type stringV string

func (v *stringV) String() string                      { return fmt.Sprint(*v) }
func (v *stringV) Assign(t *eval.Thread, o eval.Value) { *v = stringV(o.(eval.StringValue).Get(t)) }
func (v *stringV) Get(*eval.Thread) string             { return string(*v) }
func (v *stringV) Set(t *eval.Thread, x string)        { *v = stringV(x) }

func toTypeValue(val interface{}) (eval.Type, eval.Value) {
	switch val := val.(type) {
	case bool:
		r := boolV(val)
		return eval.BoolType, &r
	case uint8:
		r := uint8V(val)
		return eval.Uint8Type, &r
	case uint32:
		r := uint32V(val)
		return eval.Uint32Type, &r
	case uint:
		r := uintV(val)
		return eval.Uint64Type, &r
	case int:
		r := intV(val)
		return eval.Int64Type, &r
	case float64:
		r := float64V(val)
		return eval.Float64Type, &r
	case string:
		r := stringV(val)
		return eval.StringType, &r
	}
	logging.GetLogger().Error("toValue(%T) not implemented", val)
	return nil, nil
}
