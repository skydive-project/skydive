"use strict";
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
var __assign = (this && this.__assign) || Object.assign || function(t) {
    for (var s, i = 1, n = arguments.length; i < n; i++) {
        s = arguments[i];
        for (var p in s) if (Object.prototype.hasOwnProperty.call(s, p))
            t[p] = s[p];
    }
    return t;
};
Object.defineProperty(exports, "__esModule", { value: true });
var gapi = require("../graffiti/js/api");
var sapi = require("./api");
var api = __assign({}, gapi, sapi);
window.api = api;
window.Metadata = gapi.Metadata;
window.NE = gapi.NE;
window.GT = gapi.GT;
window.LT = gapi.LT;
window.GTE = gapi.GTE;
window.LTE = gapi.LTE;
window.IPV4RANGE = gapi.IPV4RANGE;
window.REGEX = gapi.REGEX;
window.WITHIN = gapi.WITHIN;
window.WITHOUT = gapi.WITHOUT;
window.INSIDE = gapi.INSIDE;
window.OUTSIDE = gapi.OUTSIDE;
window.BETWEEN = gapi.BETWEEN;
window.Alert = gapi.Alert;
window.Workflow = gapi.Workflow;
window.Capture = api.Capture;
window.EdgeRule = api.EdgeRule;
window.NodeRule = api.NodeRule;
window.PacketInjection = api.PacketInjection;
