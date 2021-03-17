// +build ovnk8s
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

package tests

import (
	"fmt"
	"testing"
)

// Test a pod within a deployment gets linked to its LSP
func TestOVNK8sDeployment(t *testing.T) {
	const nreplicas = 4
	test := &Test{
		setupCmds: []Cmd{
			{fmt.Sprintf("kubectl create deployment echo --image=k8s.gcr.io/echoserver:1.4 --replicas=%d", nreplicas), false},
		},

		tearDownCmds: []Cmd{
			{"kubectl delete deployment echo", false},
		},

		mode: Replay,

		checks: []CheckFunction{func(c *CheckContext) error {
			pods, err := c.gh.GetNodes(c.gremlin.V().Has("Type", "pod", "K8s.Labels.app", "echo"))
			if err != nil {
				return fmt.Errorf("Failed to find pods")
			}
			if len(pods) != nreplicas {
				return fmt.Errorf("Not enough pods in deployment")
			}
			for _, pod := range pods {
				podName, err := pod.GetFieldString("Name")
				if err != nil {
					return err
				}
				pod2lspQuery := c.gremlin.V(string(pod.ID)).OutE().Has("Manager", "ovnk8s")
				podEdges, err := c.gh.GetEdges(pod2lspQuery)
				if err != nil {
					return fmt.Errorf("Failed to find a pod edge for pod %s", podName)
				}
				if len(podEdges) != 1 {
					return fmt.Errorf("Wrong number of pod edges, expected 1 got %d", len(podEdges))
				}
				lsp, err := c.gh.GetNode(c.gremlin.V(string(podEdges[0].Child)))
				if err != nil {
					return fmt.Errorf("Failed to find a switch port for pod %s", podName)
				}
				expectedName := fmt.Sprintf("default_%s", podName)
				portName, err := lsp.GetFieldString("Name")
				if err != nil {
					return err
				}
				if portName != expectedName {
					return fmt.Errorf("LSP has wrong name. expected %s got %s", expectedName, podName)
				}
				t.Logf("POD found: %v", pod)
				t.Logf("EDGE found: %v", podEdges[0])
				t.Logf("PORT found: %v", lsp)
			}
			return nil
		}},
	}

	RunTest(t, test)
}
