/*
 * Copyright (C) 2019 Red Hat, Inc.
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

package server

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/skydive-project/skydive/js"

	"github.com/robertkrimen/otto"
	"github.com/skydive-project/skydive/api/types"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

// RegisterAPIServer exports Go functions required by the API
// to run inside the JS VM
func RegisterAPIServer(r *js.Runtime, g *graph.Graph, gremlinParser *traversal.GremlinTraversalParser, server *Server) {
	queryGremlin := func(query string) otto.Value {
		ts, err := gremlinParser.Parse(strings.NewReader(query))
		if err != nil {
			return r.MakeCustomError("ParseError", err.Error())
		}

		result, err := ts.Exec(g, false)
		if err != nil {
			return r.MakeCustomError("ExecuteError", err.Error())
		}

		source, err := result.MarshalJSON()
		if err != nil {
			return r.MakeCustomError("MarshalError", err.Error())
		}

		r, _ := r.ToValue(string(source))
		return r
	}

	r.Set("Gremlin", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return r.MakeCustomError("MissingQueryArgument", "Gremlin requires a string parameter")
		}

		query := call.Argument(0).String()

		return queryGremlin(query)
	})

	r.Set("request", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 3 || !call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
			return r.MakeCustomError("WrongArguments", "Import requires 3 string parameters")
		}

		url := call.Argument(0).String()
		method := call.Argument(1).String()
		data := []byte(call.Argument(2).String())

		subs := strings.Split(url, "/") // filepath.Base(url)
		if len(subs) < 3 {
			return r.MakeCustomError("WrongArgument", fmt.Sprintf("Malformed URL %s", url))
		}
		resource := subs[2]

		// For topology query, we directly call the Gremlin engine
		if resource == "topology" {
			query := types.TopologyParam{}
			if err := json.Unmarshal(data, &query); err != nil {
				return r.MakeCustomError("WrongArgument", fmt.Sprintf("Invalid query %s", string(data)))
			}

			return queryGremlin(query.GremlinQuery)
		}

		// This a CRUD call
		handler := server.GetHandler(resource)

		var err error
		var content interface{}

		switch method {
		case "POST":
			res := handler.New()
			if err := json.Unmarshal([]byte(data), res); err != nil {
				return r.MakeCustomError("UnmarshalError", err.Error())
			}
			if err := handler.Create(res); err != nil {
				return r.MakeCustomError("CreateError", err.Error())
			}
			b, _ := json.Marshal(res)
			content = string(b)

		case "DELETE":
			if len(subs) < 4 {
				return r.MakeCustomError("WrongArgument", "No ID specified")
			}
			handler.Delete(subs[3])

		case "GET":
			if len(subs) < 4 {
				resources := handler.Index()
				b, _ := json.Marshal(resources)
				content = string(b)
			} else {
				id := subs[3]
				obj, found := handler.Get(id)
				if !found {
					return r.MakeCustomError("NotFound", fmt.Sprintf("%s %s could not be found", resource, id))
				}
				b, _ := json.Marshal(obj)
				content = string(b)
			}
		}

		value, err := otto.ToValue(content)
		if err != nil {
			return r.MakeCustomError("WrongValue", err.Error())
		}

		return value
	})
}
