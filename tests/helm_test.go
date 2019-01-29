// +build helm

/*
 * Copyright (C) 2018 IBM, Inc.
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

package tests

import (
	"testing"
)

const (
	chartPath = "../contrib/charts"
	relName   = "skydive"
)

func TestHelmInstall(t *testing.T) {
	test := &Test{
		retries: 3,
		setupCmds: []Cmd{
			{"helm delete --purge " + relName, false},
			{"helm install --name " + relName + " " + chartPath, true},
		},
		tearDownCmds: []Cmd{
			{"helm delete --purge " + relName, true},
		},
		checks: []CheckFunction{
			func(c *CheckContext) error {
				return nil
			},
		},
	}
	RunTest(t, test)
}
