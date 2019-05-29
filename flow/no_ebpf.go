// +build !linux

/*
 * Copyright (C) 2016 Red Hat, Inc.
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

package flow

// EBPFFlow Wrapper type used for passing flows from probe to main agent routine
type EBPFFlow struct {
	kernFlow int
}

//key has already been hashed by the module, just convert to string
func kernFlowKey(kernFlow interface{}) string {
	return ""
}

func (ft *Table) newFlowFromEBPF(ebpfFlow *EBPFFlow, key string) ([]string, []*Flow) {
	return nil, nil
}

func (ft *Table) updateFlowFromEBPF(ebpfFlow *EBPFFlow, f *Flow) *Flow {
	return nil
}
