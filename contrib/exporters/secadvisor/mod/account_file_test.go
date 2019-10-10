/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"testing"
)

var accountFileConfig = []byte(`---
pipeline:
  account:
    type: file
    file:
      filename: /tmp/account.json
`)

func getAccountFile(t *testing.T) *accountFile {
	cfg := ConfigFromJSON(t, accountFileConfig)
	accounter, err := NewAccountFile(cfg)
	if err != nil {
		t.Fatalf("Account creation returned unexpected error: %s", err)
	}
	return accounter.(*accountFile)
}

func TestAccountFile(t *testing.T) {
	accounter := getAccountFile(t)

	accounter.Reset()
	accounter.Add(10)
	accounter.Add(100)

	record := accounter.read(accounter.filename)

	if record.StartupBytes != 110 {
		t.Fatalf("Expected startup-bytes 110 but got %d", record.StartupBytes)
	}

	if record.UpdateBytes != 100 {
		t.Fatalf("Expected update-bytes 100 but got %d", record.UpdateBytes)
	}
}
