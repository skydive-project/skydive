/*
 * Copyright (C) 2016 Red Hat, Inc.
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
	"bytes"
	"encoding/json"
	"time"

	shttp "github.com/skydive-project/skydive/http"
)

func UnmarshalWSMessage(msg shttp.WSMessage) (string, interface{}, error) {
	switch msg.Type {
	case "SyncRequest":
		var obj map[string]interface{}
		if msg.Obj != nil {
			decoder := json.NewDecoder(bytes.NewReader([]byte(*msg.Obj)))
			decoder.UseNumber()

			if err := decoder.Decode(&obj); err != nil {
				return "", msg, err
			}
		}

		var context GraphContext
		switch v := obj["Time"].(type) {
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return "", msg, err
			}
			unix := time.Unix(i/1000, 0).UTC()
			context.Time = &unix
		}

		return msg.Type, context, nil

	case "HostGraphDeleted":
		var obj interface{}
		if err := json.Unmarshal([]byte(*msg.Obj), &obj); err != nil {
			return "", msg, err
		}
		return msg.Type, obj, nil
	case "NodeUpdated", "NodeDeleted", "NodeAdded":
		var obj interface{}
		if err := json.Unmarshal([]byte(*msg.Obj), &obj); err != nil {
			return "", msg, err
		}

		var node Node
		if err := node.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &node, nil
	case "EdgeUpdated", "EdgeDeleted", "EdgeAdded":
		var obj interface{}
		err := json.Unmarshal([]byte(*msg.Obj), &obj)
		if err != nil {
			return "", msg, err
		}

		var edge Edge
		if err := edge.Decode(obj); err != nil {
			return "", msg, err
		}

		return msg.Type, &edge, nil
	}

	return "", msg, nil
}
