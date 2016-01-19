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
	"errors"
)

type WSMessage struct {
	Type string
	Obj  interface{}
}

func (g WSMessage) Marshal() []byte {
	j, _ := json.Marshal(g)
	return j
}

func (g WSMessage) String() string {
	return string(g.Marshal())
}

func UnmarshalWSMessage(b []byte) (WSMessage, error) {
	msg := WSMessage{}

	err := json.Unmarshal(b, &msg)
	if err != nil {
		return msg, err
	}

	if msg.Type == "SyncRequest" || msg.Type == "GetAlert" {
		return msg, nil
	}

	objMap, ok := msg.Obj.(map[string]interface{})
	if !ok {
		return msg, errors.New("Unable to parse event: " + string(b))
	}

	ID := Identifier(objMap["ID"].(string))
	metadatas := make(Metadatas)
	if m, ok := objMap["Metadatas"]; ok {
		metadatas = Metadatas(m.(map[string]interface{}))
	}

	switch msg.Type {
	/* Graph Section */
	case "SubGraphDeleted":
		fallthrough
	case "NodeUpdated":
		fallthrough
	case "NodeDeleted":
		fallthrough
	case "NodeAdded":
		if m, ok := objMap["Metadatas"]; ok {
			metadatas = Metadatas(m.(map[string]interface{}))
		}

		msg.Obj = &Node{
			graphElement: graphElement{
				ID:        ID,
				metadatas: metadatas,
			},
		}
	case "EdgeUpdated":
		fallthrough
	case "EdgeDeleted":
		fallthrough
	case "EdgeAdded":
		parent := Identifier(objMap["Parent"].(string))
		child := Identifier(objMap["Child"].(string))

		msg.Obj = &Edge{
			graphElement: graphElement{
				ID:        ID,
				metadatas: metadatas,
			},
			parent: parent,
			child:  child,
		}

		/* Alert Section */
	case "GetAlert":
		return msg, err

	}

	return msg, err
}
