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

package rest

import (
	"errors"
	"time"
)

// ErrDuplicatedResource is returned when a resource is duplicated
var ErrDuplicatedResource = errors.New("Duplicated resource")

// Resource used as interface resources for each API
type Resource interface {
	ID() string
	SetID(string)
	GetName() string
	Validate() error
}

// Handler describes resources for each API
type Handler interface {
	Name() string
	New() Resource
	Index() map[string]Resource
	Get(id string) (Resource, bool)
	Decorate(resource Resource)
	Create(resource Resource, createOpts *CreateOptions) error
	Delete(id string) error
	AsyncWatch(f WatcherCallback) StoppableWatcher
}

// CreateOptions describes the available options when creating a resource
type CreateOptions struct {
	TTL time.Duration
}

// ResourceHandler aims to creates new resource of an API
type ResourceHandler interface {
	Name() string
	New() Resource
}

// WatcherCallback callback called by the resource watcher
type WatcherCallback func(action string, id string, resource Resource)

// StoppableWatcher interface
type StoppableWatcher interface {
	Stop()
}

// ResourceWatcher asynchronous interface
type ResourceWatcher interface {
	AsyncWatch(f WatcherCallback) StoppableWatcher
}

// BasicResource is a resource with a unique identifier
// easyjson:json
// swagger:ignore
type BasicResource struct {
	UUID string `yaml:"UUID"`
}

// ID returns the resource ID
func (b *BasicResource) ID() string {
	return b.UUID
}

// SetID sets the resource ID
func (b *BasicResource) SetID(i string) {
	b.UUID = i
}

// GetName returns the resource name
func (b *BasicResource) GetName() string {
	return "BasicResource"
}

// Validate integrity of the resource
func (b *BasicResource) Validate() error {
	return nil
}
