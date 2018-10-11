/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package js

import (
	"fmt"
	"sort"
	"strings"

	"github.com/robertkrimen/otto"
)

func iterOwnAndConstructorKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	seen := make(map[string]bool)
	iterOwnKeys(vm, obj, func(prop string) {
		seen[prop] = true
		f(prop)
	})
	if cp := constructorPrototype(obj); cp != nil {
		iterOwnKeys(vm, cp, func(prop string) {
			if !seen[prop] {
				f(prop)
			}
		})
	}
}

func iterOwnKeys(vm *otto.Otto, obj *otto.Object, f func(string)) {
	Object, _ := vm.Object("Object")
	rv, _ := Object.Call("getOwnPropertyNames", obj.Value())
	gv, _ := rv.Export()
	switch gv := gv.(type) {
	case []interface{}:
		for _, v := range gv {
			f(v.(string))
		}
	case []string:
		for _, v := range gv {
			f(v)
		}
	default:
		panic(fmt.Errorf("Object.getOwnPropertyNames returned unexpected type %T", gv))
	}
}

func constructorPrototype(obj *otto.Object) *otto.Object {
	if v, _ := obj.Get("constructor"); v.Object() != nil {
		if v, _ = v.Object().Get("prototype"); v.Object() != nil {
			return v.Object()
		}
	}
	return nil
}

func getCompletions(vm *otto.Otto, line string) (results []string) {
	parts := strings.Split(line, ".")
	objRef := "this"
	prefix := line
	if len(parts) > 1 {
		objRef = strings.Join(parts[0:len(parts)-1], ".")
		prefix = parts[len(parts)-1]
	}

	obj, _ := vm.Object(objRef)
	if obj == nil {
		return nil
	}
	iterOwnAndConstructorKeys(vm, obj, func(k string) {
		if strings.HasPrefix(k, prefix) {
			if objRef == "this" {
				results = append(results, k)
			} else {
				results = append(results, strings.Join(parts[:len(parts)-1], ".")+"."+k)
			}
		}
	})

	// Append opening parenthesis (for functions) or dot (for objects)
	// if the line itRuntime is the only completion.
	if len(results) == 1 && results[0] == line {
		obj, _ := vm.Object(line)
		if obj != nil {
			if obj.Class() == "Function" {
				results[0] += "("
			} else {
				results[0] += "."
			}
		}
	}

	sort.Strings(results)
	return results
}
