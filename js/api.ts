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

declare var require: any
declare var Promise: any;
declare var $: any;
declare var global: any;
declare var Gremlin: any;
declare var request: any;
var makeRequest = null;

const Unauthorized = "Unauthorized";

import jsEnv = require('browser-or-node');
if (jsEnv) {
    var querystring = require('qs')
    var defaultsDeep = require('lodash.defaultsdeep')

    // When using node, we use najax for $.ajax calls
    if (jsEnv.isNode && !jsEnv.isBrowser) {
        var najax = require('najax');
        global.$ = <any>{ "ajax": najax };
    }

    makeRequest = function (client: Client, url: string, method: string, data: string, opts: Object) : any {
        return new Promise(function (resolve, reject) {
            return $.ajax(defaultsDeep(opts, {
                dataType: "json",
                url: client.baseURL + url,
                data: data,
                contentType: "application/json; charset=utf-8",
                method: method
            }))
            .fail(function (jqXHR) {
              if (jqXHR.status == 401) {
                  reject(Unauthorized)
              } else {
                  reject(jqXHR.statusText)
              }
            })
            .done(function (data, statusText, jqXHR) {
                if (jqXHR.status == 200) {
                    var cookie = jqXHR.getResponseHeader("Set-Cookie");
                    if (cookie) {
                        client.cookie = cookie;
                    }
                    resolve(data, jqXHR);
                } else {
                    reject(jqXHR.statusText);
                }
            });
        })
    }
} else {
    // We're running inside Otto. We use the 'request' method
    // exported by Skydive that makes use of the REST client
    // so we get support for authentication
    makeRequest = function (client: Client, url: string, method: string, body: string, opts: Object) : any {
        var data: any
        var output = request(url, method, body)
        if (output && output.length > 0) {
            data = JSON.parse(output)
        } else {
            data = null
        }
        return {'then': function() { return data; } }
    }
}

function paramsString(params: any[]): string {
  var s = ""
  for (let param in params) {
      if (param !== "0") {
          s += ", "
      }
      switch (typeof params[param]) {
          case "string":
              s += '"' + params[param] + '"';
              break;
          default:
              s += params[param];
              break;
      }
    }
    return s
}

// Returns a string of the invocation of 'method' with its parameters 'params'
// like MyMethod(1, 2, "s")
function callString(method: string, ...params: any[]): string {
    let s = method + "("
    s += paramsString(params)
    s += ")"
    return s
}

export class SerializationHelper {
    static toInstance<T>(obj: T, jsonObj: Object) : T {
        if (typeof obj["fromJSON"] === "function") {
            obj["fromJSON"](jsonObj);
        }
        else {
            for (var propName in jsonObj) {
                obj[propName] = jsonObj[propName]
            }
        }

        return obj;
    }

    static unmarshalArray<T>(data, c: new () => T): T[] {
        var items: T[] = [];
        for (var obj in data) {
            let newT: T = SerializationHelper.toInstance(new c(), data[obj]);
            items.push(newT);
        }
        return items
    }

    static unmarshalMapArray<T>(data, c: new () => T): { [key: string]: T[]; } {
        var items: { [key: string]: T[]; } = {};
        for (var key in data) {
            items[key] = this.unmarshalArray(data[key], c);
        }
        return items
    }

    static unmarshalMap<T>(data, c: new () => T): { [key: string]: T; } {
        var items: { [key: string]: T; } = {};
        for (var obj in data) {
            let newT: T = SerializationHelper.toInstance(new c(), data[obj]);
            items[obj] = newT;
        }
        return items
    }
}

export class APIObject extends Object {
}

export class Alert extends APIObject {
}

export class Capture extends APIObject {
}

export class EdgeRule extends APIObject {
}

export class NodeRule extends APIObject {
}

export class PacketInjection extends APIObject {
}

export class Workflow extends APIObject {
}

export class API<T extends APIObject> {
    Resource: string
    Factory: new () => T
    client: Client

    constructor(client: Client, resource: string, c: new () => T) {
        this.client = client;
        this.Resource = resource;
        this.Factory = c;
    }

    create(obj: T) {
        let resource = this.Resource;
        return this.client.request("/api/" + resource, "POST", JSON.stringify(obj), {})
            .then(function (data) {
                return SerializationHelper.toInstance(new (<any>obj.constructor)(), data);
            })
    }

    list() {
        let resource = this.Resource;
        let factory = this.Factory;

        return this.client.request('/api/' + resource, "GET", "", {})
            .then(function (data) {
                let resources = {};
                for (var obj in data) {
                    let resource = SerializationHelper.toInstance(new factory(), data[obj]);
                    resources[obj] = resource
                }
                return resources
            });
    }

    get(id) {
        let resource = this.Resource;
        let factory = this.Factory;
        return this.client.request('/api/' + resource + "/" + id, "GET", "", {})
            .then(function (data) {
                return SerializationHelper.toInstance(new factory(), data)
            })
    }

    delete(id: string) {
        let resource = this.Resource;
        return this.client.request('/api/' + resource + "/" + id, "DELETE", "", { "dataType": "" })
    }
}

export class GraphElement {
}

export class GraphNode extends GraphElement {
}

export class GraphEdge extends GraphElement {
}

type NodeMap = { [key: string]: GraphNode; };
type EdgeMap = { [key: string]: GraphEdge; };

export class Graph {
    nodes: NodeMap;
    edges: EdgeMap;

    constructor(nodes: NodeMap, edges: EdgeMap) {
        this.nodes = nodes || {}
        this.edges = edges || {}
    }
}

export class GremlinAPI {
    client: Client

    constructor(client: Client) {
        this.client = client;
    }

    query(s: string) {
        return this.client.request('/api/topology', "POST", JSON.stringify({'GremlinQuery': s}), {})
            .then(function (data) {
                if (data === null)
                    return [];
                // Result can be [Node] or [[Node, Node]]
                if (data.length > 0 && data[0] instanceof Array)
                    data = data[0];
                return data
            });
    }

    G() : G {
        return new G(this)
    }
}

export class _Metadata {
    params: any[]

    constructor(...params: any[]) {
      this.params = params
    }

    public toString(): string {
        return "Metadata(" + paramsString(this.params) + ")"
    }
}

export function Metadata(...params: any[]) : _Metadata {
  return new _Metadata(...params)
}

export interface Step {
    name(): string
    serialize(data): any
}

export class Step implements Step {
    api: GremlinAPI
    gremlin: string
    previous: Step
    params: any[]

    name() { return "InvalidStep"; }

    constructor(api: GremlinAPI, previous: Step = null, ...params: any[]) {
        this.api = api
        this.previous = previous
        this.params = params
    }

    public toString(): string {
        if (this.previous !== null) {
            let gremlin = this.previous.toString() + ".";
            return gremlin + callString(this.name(), ...this.params)
        } else {
            return this.name()
        }
    }

    serialize(data) {
        return this.previous.serialize(data);
    }

    result() {
        let data = this.api.query(this.toString())
        return this.serialize(data)
    }
}

export class G extends Step {
    name() { return "G" }

    constructor(api: GremlinAPI) {
        super(api)
    }

    serialize(data) {
        return {
            "Nodes": SerializationHelper.unmarshalArray(data[0].Nodes, GraphNode),
            "Edges": SerializationHelper.unmarshalArray(data[0].Edges, GraphEdge),
        };
    }

    Context(...params: any[]): G {
        return new Context(this.api);
    }

    V(...params: any[]): V {
        return new V(this.api, this, ...params);
    }

    E(...params: any[]): E {
        return new E(this.api, this, ...params);
    }

    Flows(...params: any[]): Flows {
        return new Flows(this.api, this, ...params);
    }
}

export class Context extends G {
    name() { return "Context" }

    constructor(api: GremlinAPI, previous: Step = null, ...params: any[]) {
        super(api)
        this.previous = previous
        this.params = params
    }
}

export class Subgraph extends G {
    name() { return "Subgraph" }

    constructor(api: GremlinAPI, previous: Step = null) {
        super(api)
        this.previous = previous
    }
}

type Constructor<T> = new(...args: any[]) => T;

function MixinStep<T extends Constructor<{}>>(Base: T, name: string) {
    return class extends Base {
        name() { return name }
        constructor(...args: any[]) {
            super(...args);
        }
    }
}

export class V extends Step {
    name() { return "V" }

    serialize(data) {
        return SerializationHelper.unmarshalArray(data, GraphNode);
    }

    Has(...params: any[]): V {
        return new HasV(this.api, this, ...params);
    }

    HasEither(...params: any[]): V {
        return new HasEitherV(this.api, this, ...params);
    }

    HasKey(...params: any[]): V {
        return new HasKeyV(this.api, this, ...params);
    }

    HasNot(...params: any[]): V {
        return new HasNotV(this.api, this, ...params);
    }

    Dedup(...params: any[]): V {
        return new DedupV(this.api, this, ...params);
    }

    In(...params: any[]): V {
        return new In(this.api, this, ...params);
    }

    Out(...params: any[]): V {
        return new Out(this.api, this, ...params);
    }

    Both(...params: any[]): V {
        return new Both(this.api, this, ...params);
    }

    Range(...params: any[]): V {
        return new RangeV(this.api, this, ...params);
    }

    Limit(...params: any[]): V {
        return new LimitV(this.api, this, ...params);
    }

    InE(...params: any[]): E {
        return new InE(this.api, this, ...params);
    }

    OutE(...params: any[]): E {
        return new OutE(this.api, this, ...params);
    }

    BothE(...params: any[]): E {
        return new BothE(this.api, this, ...params);
    }

    ShortestPathTo(...params: any[]): ShortestPath {
        return new ShortestPath(this.api, this, ...params);
    }

    Subgraph(): G {
        return new Subgraph(this.api, this);
    }

    Count(): Value {
        return new Count(this.api, this);
    }

    Values(...params: any[]): Value {
        return new Values(this.api, this, ...params);
    }

    Keys(...params: any[]): Value {
        return new Keys(this.api, this, ...params);
    }

    Sum(...params: any[]): Value {
        return new Keys(this.api, this, ...params);
    }

    Sort(...params: any[]): V {
        return new SortV(this.api, this, ...params);
    }

    Metrics(...params: any[]): Metrics {
        return new Metrics(this.api, this, ...params);
    }

    Sockets(...params: any[]): Sockets {
        return new Sockets(this.api, this, ...params);
    }

    Flows(...params: any[]): Flows {
        return new Flows(this.api, this, ...params)
    }
}

export class E extends Step {
    name() { return "E" }

    serialize(data) {
        return SerializationHelper.unmarshalArray(data, GraphEdge);
    }

    Has(...params: any[]): E {
        return new HasE(this.api, this, ...params);
    }

    HasEither(...params: any[]): E {
        return new HasEitherE(this.api, this, ...params);
    }

    HasKey(...params: any[]): E {
        return new HasKeyE(this.api, this, ...params);
    }

    HasNot(...params: any[]): E {
        return new HasNotE(this.api, this, ...params);
    }

    Dedup(...params: any[]): E {
        return new DedupE(this.api, this, ...params);
    }

    Range(...params: any[]): E {
        return new RangeE(this.api, this, ...params);
    }

    Limit(...params: any[]): E {
        return new LimitE(this.api, this, ...params);
    }

    InV(...params: any[]): V {
        return new InV(this.api, this, ...params);
    }

    OutV(...params: any[]): V {
        return new OutV(this.api, this, ...params);
    }

    BothV(...params: any[]): V {
        return new BothV(this.api, this, ...params);
    }

    Subgraph(): G {
        return new Subgraph(this.api, this);
    }

    Count(): Value {
        return new Count(this.api, this);
    }

    Values(...params: any[]): Value {
        return new Values(this.api, this, ...params);
    }

    Keys(...params: any[]): Value {
        return new Keys(this.api, this, ...params);
    }

    Sort(...params: any[]): E {
        return new SortE(this.api, this, ...params);
    }
}

class HasV extends MixinStep(V, "Has") { }
class HasE extends MixinStep(E, "Has") { }

class HasEitherV extends MixinStep(V, "HasEither") { }
class HasEitherE extends MixinStep(E, "HasEither") { }

class HasKeyV extends MixinStep(V, "HasKey") { }
class HasKeyE extends MixinStep(E, "HasKey") { }

class HasNotV extends MixinStep(V, "HasNot") { }
class HasNotE extends MixinStep(E, "HasNot") { }

class DedupV extends MixinStep(V, "Dedup") { }
class DedupE extends MixinStep(E, "Dedup") { }

class RangeV extends MixinStep(V, "Range") { }
class RangeE extends MixinStep(E, "Range") { }

class LimitV extends MixinStep(V, "Limit") { }
class LimitE extends MixinStep(E, "Limit") { }

class SortV extends MixinStep(V, "Sort") { }
class SortE extends MixinStep(E, "Sort") { }

export class Out extends V {
    name() { return "Out" }
}

export class In extends V {
    name() { return "In" }
}

export class Both extends V {
    name() { return "Both" }
}

export class OutV extends V {
    name() { return "OutV" }
}

export class InV extends V {
    name() { return "InV" }
}

export class OutE extends E {
    name() { return "OutE" }
}

export class InE extends E {
    name() { return "InE" }
}

export class BothE extends E {
    name() { return "BothE" }
}

export class BothV extends V {
    name() { return "BothV" }
}

export class ShortestPath extends Step {
    name() { return "ShortestPathTo" }

    public toString(): string {
        if (this.previous !== null) {
            let gremlin = this.previous.toString() + ".";
            return gremlin + callString(this.name(), ...this.params)
        } else {
            return this.name()
        }
    }

    serialize(data) {
        var items: GraphNode[][] = [];
        for (var obj in data) {
            items.push(SerializationHelper.unmarshalArray(data[obj], GraphNode));
        }
        return items;
    }
}

class Predicate {
    name: string
    params: any

    constructor(name: string, ...params: any[]) {
        this.name = name
        this.params = params
    }

    public toString(): string {
        return callString(this.name, ...this.params)
    }
}

export function NE(param: any): Predicate {
    return new Predicate("NE", param)
}

export function GT(param: any): Predicate {
    return new Predicate("GT", param)
}

export function LT(param: any): Predicate {
    return new Predicate("LT", param)
}

export function GTE(param: any): Predicate {
    return new Predicate("GTE", param)
}

export function LTE(param: any): Predicate {
    return new Predicate("LTE", param)
}

export function IPV4RANGE(param: any): Predicate {
    return new Predicate("IPV4RANGE", param)
}

export function REGEX(param: any): Predicate {
    return new Predicate("REGEX", param)
}

export function WITHIN(...params: any[]): Predicate {
    return new Predicate("WITHIN", ...params)
}

export function WITHOUT(...params: any[]): Predicate {
    return new Predicate("WITHOUT", ...params)
}

export function INSIDE(from, to: any): Predicate {
    return new Predicate("INSIDE", from, to)
}

export function OUTSIDE(from, to: any): Predicate {
    return new Predicate("OUTSIDE", from, to)
}

export function BETWEEN(from, to: any): Predicate {
    return new Predicate("BETWEEN", from, to)
}

export var FOREVER = new Predicate("FOREVER")
export var NOW = new Predicate("NOW")
export var DESC = new Predicate("DESC")
export var ASC = new Predicate("ASC")

export class Metric {
}

export class FlowLayer {
}

export class ICMPLayer {
}

export class FlowMetric {
}

export class RawPacket {
}

export class TCPMetric {
}

export class Flow {
}

export class Flows extends Step {
    name() { return "Flows" }

    serialize(data) {
        return SerializationHelper.unmarshalArray(data, Flow);
    }

    Has(...params: any[]): Flows {
        return new HasFlows(this.api, this, ...params);
    }

    HasEither(...params: any[]): Flows {
        return new HasEitherFlows(this.api, this, ...params);
    }

    Count(...params: any[]): Value {
        return new Count(this.api, this);
    }

    Values(...params: any[]): Value {
        return new Values(this.api, this, ...params);
    }

    Keys(...params: any[]): Value {
        return new Keys(this.api, this, ...params);
    }

    Sum(...params: any[]): Value {
        return new Keys(this.api, this, ...params);
    }

    Sort(...params: any[]): Flows {
        return new SortFlows(this.api, this, ...params);
    }

    Out(...params: any[]): V {
        return new Out(this.api, this, ...params);
    }

    In(...params: any[]): V {
        return new In(this.api, this, ...params);
    }

    Both(...params: any[]): V {
        return new Both(this.api, this, ...params);
    }

    Nodes(...params: any[]): V {
        return new Nodes(this.api, this, ...params);
    }

    Hops(...params: any[]): V {
        return new Hops(this.api, this, ...params);
    }

    CaptureNode(...params: any[]): V {
        return new CaptureNode(this.api, this, ...params);
    }

    Metrics(...params: any[]): Metrics {
        return new Metrics(this.api, this, ...params);
    }

    RawPackets(...params: any[]): RawPackets {
        return new RawPackets(this.api, this, ...params);
    }

    Sockets(...params: any[]): Sockets {
        return new Sockets(this.api, this, ...params);
    }

    Dedup(...params: any[]): Flows {
        return new DedupFlows(this.api, this, ...params);
    }
}

class HasFlows extends MixinStep(Flows, "Has") { }
class HasEitherFlows extends MixinStep(Flows, "HasEither") { }
class SortFlows extends MixinStep(Flows, "Sort") { }
class DedupFlows extends MixinStep(Flows, "Dedup") { }

export class Nodes extends V {
    name() { return "Nodes" }
}

export class Hops extends V {
    name() { return "Hops" }
}

export class CaptureNode extends V {
    name() { return "CaptureNode" }
}

export class Value extends Step {
    serialize(data) {
        return data;
    }
}

export class Count extends Value {
    name() { return "Count" }
}

export class Values extends Value {
    name() { return "Values" }
}

export class Keys extends Value {
    name() { return "Keys" }
}

export class Sum extends Value {
    name() { return "Sum" }
}

export class Metrics extends Step {
    name() { return "Metrics" }

    serialize(data) {
        return SerializationHelper.unmarshalMapArray(data[0], Metric);
    }

    Sum(...params: any[]): Value {
        return new Sum(this.api, this, ...params);
    }

    Aggregates(...params: any[]): Metrics {
        return new Aggregates(this.api, this, ...params);
    }

    Count(): Value {
        return new Count(this.api, this);
    }
}

export class Aggregates extends Metrics {
    name() { return "Aggregates" }
}

export class RawPackets extends Step {
    name() { return "RawPackets" }

    serialize(data) {
        return SerializationHelper.unmarshalMapArray(data[0], RawPacket);
    }

    BPF(...params: any[]): RawPackets {
        return new BPF(this.api, this, ...params);
    }
}

export class BPF extends RawPackets {
    name() { return "BPF" }
}

class Socket {
}

export class Sockets extends Step {
    name() { return "Sockets" }

    serialize(data) {
        return SerializationHelper.unmarshalMapArray(data[0], Socket);
    }

    Values(...params: any[]): Value {
        return new Values(this.api, this, ...params);
    }

    Has(...params: any[]): Sockets {
        return new HasSockets(this.api, this, ...params);
    }
}

class HasSockets extends MixinStep(Sockets, "Has") { }

export class Client {
    baseURL: string
    username: string
    password: string
    cookie: string
    alerts: API<Alert>
    captures: API<Capture>
    edgeRules: API<EdgeRule>
    nodeRules: API<NodeRule>
    packetInjections: API<PacketInjection>
    workflows: API<Workflow>
    gremlin: GremlinAPI
    G: G

    constructor(options: Object) {
        options = options || {};
        this.baseURL = options["baseURL"] || "";
        this.username = options["username"] || "";
        this.password = options["password"] || "";
        this.cookie = options["cookie"] || "";

        this.alerts = new API(this, "alert", Alert);
        this.captures = new API(this, "capture", Capture);
        this.packetInjections = new API(this, "injectpacket", PacketInjection);
        this.nodeRules = new API(this, "noderule", NodeRule);
        this.edgeRules = new API(this, "edgerule", EdgeRule);
        this.workflows = new API(this, "workflow", Workflow);
        this.gremlin = new GremlinAPI(this);
        this.G = this.gremlin.G();
    }

    login() {
        var self = this;
        if (this.username && this.password) {
            return makeRequest(this, "/login", "POST",
                               querystring.stringify({"username": this.username, "password": this.password}),
                               { "contentType": "application/x-www-form-urlencoded",
                                 "dataType": "" })
        }
        return Promise.resolve();
    }

    request(url: string, method: string, data: any, opts: Object): any {
        if (this.cookie) {
          opts["headers"] = { "Cookie": this.cookie }
        }
        return makeRequest(this, url, method, data, opts);
    }
}
