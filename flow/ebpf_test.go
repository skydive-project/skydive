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

package flow

import (
	"testing"
	"time"

	"github.com/skydive-project/skydive/graffiti/service"
)

func TestEBPFFlow(t *testing.T) {
	expired := make(chan interface{}, 10)
	defer close(expired)

	table := NewTable(time.Minute, time.Hour, &fakeMessageSender{}, UUIDs{}, TableOpts{})
	_, extFlowChan, _ := table.Start(expired)

	for table.State() != service.RunningState {
		time.Sleep(100 * time.Millisecond)
	}

	extFlowChan <- &ExtFlow{Type: EBPFExtFlowType, Obj: newEBPFFlow(222, "01:23:00:00:00:00", "")}
	extFlowChan <- &ExtFlow{Type: EBPFExtFlowType, Obj: newEBPFFlow(444, "", "04:56:00:00:00:00")}
	extFlowChan <- &ExtFlow{Type: EBPFExtFlowType, Obj: newEBPFFlow(666, "", "01:23:00:00:00:00")}

	table.Stop()

	if len(expired) != 3 {
		t.Errorf("Table should have expired 3 flows, got : %d", len(expired))
	}
}
