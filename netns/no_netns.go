// +build !linux

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

package netns

import "github.com/skydive-project/skydive/common"

// Context describes a NameSpace Context switch API
type Context struct {
}

// Quit the NameSpace and go back to the original one
func (n *Context) Quit() error {
	return nil
}

// Close the NameSpace
func (n *Context) Close() {
}

// NewContext creates a new NameSpace context base on path
func NewContext(path string) (*Context, error) {
	if path != "" {
		return nil, common.ErrNotImplemented
	}

	return &Context{}, nil
}
