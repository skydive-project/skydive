/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package traversal

import "github.com/skydive-project/skydive/graffiti/graph/traversal"

const (
	traversalFlowToken        traversal.Token = 1001
	traversalHopsToken        traversal.Token = 1002
	traversalNodesToken       traversal.Token = 1003
	traversalCaptureNodeToken traversal.Token = 1004
	traversalAggregatesToken  traversal.Token = 1005
	traversalRawPacketsToken  traversal.Token = 1006
	traversalBpfToken         traversal.Token = 1007
	traversalMetricsToken     traversal.Token = 1008
	traversalSocketsToken     traversal.Token = 1009
	traversalDescendantsToken traversal.Token = 1010
	traversalNextHopToken     traversal.Token = 1011
	traversalGroupToken       traversal.Token = 1012
	traversalMoreThanToken    traversal.Token = 1013
	traversalAscendantsToken  traversal.Token = 1014
)
