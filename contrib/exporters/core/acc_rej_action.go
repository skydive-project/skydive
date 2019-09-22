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

type actionerNone struct {
}

// NewAccRejNone create a new Accept/Reject process that will do nothing
func NewActionerNone(cfg *viper.Viper) (interface{}, error) {
	return &actionerNone{}, nil
}

// Action for type accRejNone does nothing
func (t actionerNone) Action(f interface{}) interface{} {
	return f
}
