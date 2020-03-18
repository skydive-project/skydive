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
	api "github.com/skydive-project/skydive/graffiti/api/server"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/graph/traversal"
)

// NewWorkflowRuntime returns a new Workflow runtime
func NewWorkflowRuntime(g *graph.Graph, tr *traversal.GremlinTraversalParser, server *api.Server) (*js.Runtime, error) {
	runtime, err := js.NewRuntime()
	if err != nil {
		return nil, err
	}
	runtime.Start()

	queryGremlin := func(query string) otto.Value {
		ts, err := tr.Parse(strings.NewReader(query))
		if err != nil {
			return runtime.MakeCustomError("ParseError", err.Error())
		}

		result, err := ts.Exec(g, false)
		if err != nil {
			return runtime.MakeCustomError("ExecuteError", err.Error())
		}

		source, err := result.MarshalJSON()
		if err != nil {
			return runtime.MakeCustomError("MarshalError", err.Error())
		}

		r, _ := runtime.ToValue(string(source))
		return r
	}

	runtime.Set("Gremlin", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 1 || !call.Argument(0).IsString() {
			return runtime.MakeCustomError("MissingQueryArgument", "Gremlin requires a string parameter")
		}

		query := call.Argument(0).String()

		return queryGremlin(query)
	})

	runtime.Set("request", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) < 3 || !call.Argument(0).IsString() || !call.Argument(1).IsString() || !call.Argument(2).IsString() {
			return runtime.MakeCustomError("WrongArguments", "Import requires 3 string parameters")
		}

		url := call.Argument(0).String()
		method := call.Argument(1).String()
		data := []byte(call.Argument(2).String())

		subs := strings.Split(url, "/") // filepath.Base(url)
		if len(subs) < 3 {
			return runtime.MakeCustomError("WrongArgument", fmt.Sprintf("Malformed URL %s", url))
		}
		resource := subs[2]

		// For topology query, we directly call the Gremlin engine
		if resource == "topology" {
			query := types.TopologyParams{}
			if err := json.Unmarshal(data, &query); err != nil {
				return runtime.MakeCustomError("WrongArgument", fmt.Sprintf("Invalid query %s", string(data)))
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
				return runtime.MakeCustomError("UnmarshalError", err.Error())
			}
			if err := handler.Create(res, nil); err != nil {
				return runtime.MakeCustomError("CreateError", err.Error())
			}
			b, _ := json.Marshal(res)
			content = string(b)

		case "DELETE":
			if len(subs) < 4 {
				return runtime.MakeCustomError("WrongArgument", "No ID specified")
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
					return runtime.MakeCustomError("NotFound", fmt.Sprintf("%s %s could not be found", resource, id))
				}
				b, _ := json.Marshal(obj)
				content = string(b)
			}
		}

		value, err := otto.ToValue(content)
		if err != nil {
			return runtime.MakeCustomError("WrongValue", err.Error())
		}

		return value
	})

	return runtime, nil
}
