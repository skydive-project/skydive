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

package core

import (
	"github.com/spf13/viper"
)

type accountRecord struct {
	bytes int64
}

type accountNone struct {
}

// Reset counters
func (a *accountNone) Cleanup() {
}

// Add to counters
func (a *accountNone) Account(bytes int64) {
}

// NewAccountNone create a new accounter
func NewAccountNone(cfg *viper.Viper) (interface{}, error) {
	return &accountNone{}, nil
}
