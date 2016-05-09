/*
 * Copyright (C) 2016 Red Hat, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package flow

import (
	"time"
)

type ExpireUpdateFunc func(f []*Flow)

type tableManagerAsyncFunction func(fn ExpireUpdateFunc, updateFrom int64)

type tableManagerAsyncParam struct {
	function tableManagerAsyncFunction
	callback ExpireUpdateFunc
	every    time.Duration
	duration time.Duration
}

type tableManagerAsync struct {
	tableManagerAsyncParam
	ticker  *time.Ticker
	running bool
}

type tableManager struct {
	expire, updated tableManagerAsync
}

func (ftma *tableManagerAsync) Register(p *tableManagerAsyncParam) {
	ftma.tableManagerAsyncParam = *p
	ftma.ticker = time.NewTicker(ftma.every)
	ftma.running = true
}

func (ftma *tableManagerAsync) Unregister() {
	ftma.ticker.Stop()
	ftma.running = false
}
