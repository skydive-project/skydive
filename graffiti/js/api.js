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
var __extends = (this && this.__extends) || (function () {
    var extendStatics = Object.setPrototypeOf ||
        ({ __proto__: [] } instanceof Array && function (d, b) { d.__proto__ = b; }) ||
        function (d, b) { for (var p in b) if (b.hasOwnProperty(p)) d[p] = b[p]; };
    return function (d, b) {
        extendStatics(d, b);
        function __() { this.constructor = d; }
        d.prototype = b === null ? Object.create(b) : (__.prototype = b.prototype, new __());
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
var makeRequest = null;
var Unauthorized = "Unauthorized";
var jsEnv = require("browser-or-node");
if (jsEnv) {
    var querystring = require('qs');
    var defaultsDeep = require('lodash.defaultsdeep');
    // When using node, we use najax for $.ajax calls
    if (jsEnv.isNode && !jsEnv.isBrowser) {
        var najax = require('najax');
        global.$ = { "ajax": najax };
    }
    makeRequest = function (client, url, method, data, opts) {
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
                    reject(Unauthorized);
                }
                else {
                    reject(jqXHR);
                }
            })
                .done(function (data, statusText, jqXHR) {
                if (jqXHR.status >= 200 && jqXHR.status < 400) {
                    var cookie = jqXHR.getResponseHeader("Set-Cookie");
                    if (cookie) {
                        client.cookie = cookie;
                    }
                    resolve(data, jqXHR);
                }
                else {
                    reject(jqXHR);
                }
            });
        });
    };
}
else {
    // We're running inside Otto. We use the 'request' method
    // exported by Skydive that makes use of the REST client
    // so we get support for authentication
    makeRequest = function (client, url, method, body, opts) {
        var data;
        var output = request(url, method, body);
        if (output && output.length > 0) {
            data = JSON.parse(output);
        }
        else {
            data = null;
        }
        return { 'then': function () { return data; } };
    };
}
function paramsString(params) {
    var s = "";
    for (var param in params) {
        if (param !== "0") {
            s += ", ";
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
    return s;
}
// Returns a string of the invocation of 'method' with its parameters 'params'
// like MyMethod(1, 2, "s")
function callString(method) {
    var params = [];
    for (var _i = 1; _i < arguments.length; _i++) {
        params[_i - 1] = arguments[_i];
    }
    var s = method + "(";
    s += paramsString(params);
    s += ")";
    return s;
}
var SerializationHelper = /** @class */ (function () {
    function SerializationHelper() {
    }
    SerializationHelper.toInstance = function (obj, jsonObj) {
        if (typeof obj["fromJSON"] === "function") {
            obj["fromJSON"](jsonObj);
        }
        else {
            for (var propName in jsonObj) {
                obj[propName] = jsonObj[propName];
            }
        }
        return obj;
    };
    SerializationHelper.unmarshalArray = function (data, c) {
        var items = [];
        for (var obj in data) {
            var newT = SerializationHelper.toInstance(new c(), data[obj]);
            items.push(newT);
        }
        return items;
    };
    SerializationHelper.unmarshalMapArray = function (data, c) {
        var items = {};
        for (var key in data) {
            items[key] = this.unmarshalArray(data[key], c);
        }
        return items;
    };
    SerializationHelper.unmarshalMap = function (data, c) {
        var items = {};
        for (var obj in data) {
            var newT = SerializationHelper.toInstance(new c(), data[obj]);
            items[obj] = newT;
        }
        return items;
    };
    return SerializationHelper;
}());
exports.SerializationHelper = SerializationHelper;
var APIObject = /** @class */ (function (_super) {
    __extends(APIObject, _super);
    function APIObject() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return APIObject;
}(Object));
exports.APIObject = APIObject;
var Alert = /** @class */ (function (_super) {
    __extends(Alert, _super);
    function Alert() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return Alert;
}(APIObject));
exports.Alert = Alert;
var Workflow = /** @class */ (function (_super) {
    __extends(Workflow, _super);
    function Workflow() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return Workflow;
}(APIObject));
exports.Workflow = Workflow;
var API = /** @class */ (function () {
    function API(client, resource, c) {
        this.client = client;
        this.Resource = resource;
        this.Factory = c;
    }
    API.prototype.create = function (obj) {
        var resource = this.Resource;
        return this.client.request("/api/" + resource, "POST", JSON.stringify(obj), {})
            .then(function (data) {
            return SerializationHelper.toInstance(new obj.constructor(), data);
        });
    };
    API.prototype.list = function () {
        var resource = this.Resource;
        var factory = this.Factory;
        return this.client.request('/api/' + resource, "GET", "", {})
            .then(function (data) {
            var resources = {};
            for (var obj in data) {
                var resource_1 = SerializationHelper.toInstance(new factory(), data[obj]);
                resources[obj] = resource_1;
            }
            return resources;
        });
    };
    API.prototype.get = function (id) {
        var resource = this.Resource;
        var factory = this.Factory;
        return this.client.request('/api/' + resource + "/" + id, "GET", "", {})
            .then(function (data) {
            return SerializationHelper.toInstance(new factory(), data);
        });
    };
    API.prototype.delete = function (id) {
        var resource = this.Resource;
        return this.client.request('/api/' + resource + "/" + id, "DELETE", "", { "dataType": "" });
    };
    return API;
}());
exports.API = API;
var GraphElement = /** @class */ (function () {
    function GraphElement() {
    }
    return GraphElement;
}());
exports.GraphElement = GraphElement;
var GraphNode = /** @class */ (function (_super) {
    __extends(GraphNode, _super);
    function GraphNode() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return GraphNode;
}(GraphElement));
exports.GraphNode = GraphNode;
var GraphEdge = /** @class */ (function (_super) {
    __extends(GraphEdge, _super);
    function GraphEdge() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return GraphEdge;
}(GraphElement));
exports.GraphEdge = GraphEdge;
var Graph = /** @class */ (function () {
    function Graph(nodes, edges) {
        this.nodes = nodes || {};
        this.edges = edges || {};
    }
    return Graph;
}());
exports.Graph = Graph;
var GremlinAPI = /** @class */ (function () {
    function GremlinAPI(client) {
        this.client = client;
    }
    GremlinAPI.prototype.query = function (s) {
        return this.client.request('/api/topology', "POST", JSON.stringify({ 'GremlinQuery': s }), {})
            .then(function (data) {
            if (data === null)
                return [];
            // Result can be [Node] or [[Node, Node]]
            if (data.length > 0 && data[0] instanceof Array)
                data = data[0];
            return data;
        });
    };
    GremlinAPI.prototype.G = function () {
        return new G(this);
    };
    return GremlinAPI;
}());
exports.GremlinAPI = GremlinAPI;
var _Metadata = /** @class */ (function () {
    function _Metadata() {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        this.params = params;
    }
    _Metadata.prototype.toString = function () {
        return "Metadata(" + paramsString(this.params) + ")";
    };
    return _Metadata;
}());
exports._Metadata = _Metadata;
function Metadata() {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (_Metadata.bind.apply(_Metadata, [void 0].concat(params)))();
}
exports.Metadata = Metadata;
var Step = /** @class */ (function () {
    function Step(api, previous) {
        if (previous === void 0) { previous = null; }
        var params = [];
        for (var _i = 2; _i < arguments.length; _i++) {
            params[_i - 2] = arguments[_i];
        }
        this.api = api;
        this.previous = previous;
        this.params = params;
    }
    Step.prototype.name = function () { return "InvalidStep"; };
    Step.prototype.toString = function () {
        if (this.previous !== null) {
            var gremlin = this.previous.toString() + ".";
            return gremlin + callString.apply(void 0, [this.name()].concat(this.params));
        }
        else {
            return this.name();
        }
    };
    Step.prototype.serialize = function (data) {
        return this.previous.serialize(data);
    };
    Step.prototype.result = function () {
        var data = this.api.query(this.toString());
        return this.serialize(data);
    };
    return Step;
}());
exports.Step = Step;
var G = /** @class */ (function (_super) {
    __extends(G, _super);
    function G(api) {
        return _super.call(this, api) || this;
    }
    G.prototype.name = function () { return "G"; };
    G.prototype.serialize = function (data) {
        return {
            "Nodes": SerializationHelper.unmarshalArray(data[0].Nodes, GraphNode),
            "Edges": SerializationHelper.unmarshalArray(data[0].Edges, GraphEdge),
        };
    };
    G.prototype.Context = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new Context(this.api);
    };
    G.prototype.V = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (V.bind.apply(V, [void 0, this.api, this].concat(params)))();
    };
    G.prototype.E = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (E.bind.apply(E, [void 0, this.api, this].concat(params)))();
    };
    return G;
}(Step));
exports.G = G;
var Context = /** @class */ (function (_super) {
    __extends(Context, _super);
    function Context(api, previous) {
        if (previous === void 0) { previous = null; }
        var params = [];
        for (var _i = 2; _i < arguments.length; _i++) {
            params[_i - 2] = arguments[_i];
        }
        var _this = _super.call(this, api) || this;
        _this.previous = previous;
        _this.params = params;
        return _this;
    }
    Context.prototype.name = function () { return "Context"; };
    return Context;
}(G));
exports.Context = Context;
var Subgraph = /** @class */ (function (_super) {
    __extends(Subgraph, _super);
    function Subgraph(api, previous) {
        if (previous === void 0) { previous = null; }
        var _this = _super.call(this, api) || this;
        _this.previous = previous;
        return _this;
    }
    Subgraph.prototype.name = function () { return "Subgraph"; };
    return Subgraph;
}(G));
exports.Subgraph = Subgraph;
function MixinStep(Base, name) {
    return /** @class */ (function (_super) {
        __extends(class_1, _super);
        function class_1() {
            var args = [];
            for (var _i = 0; _i < arguments.length; _i++) {
                args[_i] = arguments[_i];
            }
            return _super.apply(this, args) || this;
        }
        class_1.prototype.name = function () { return name; };
        return class_1;
    }(Base));
}
var V = /** @class */ (function (_super) {
    __extends(V, _super);
    function V() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    V.prototype.name = function () { return "V"; };
    V.prototype.serialize = function (data) {
        return SerializationHelper.unmarshalArray(data, GraphNode);
    };
    V.prototype.Has = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasV.bind.apply(HasV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.HasEither = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasEitherV.bind.apply(HasEitherV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.HasKey = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasKeyV.bind.apply(HasKeyV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.HasNot = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasNotV.bind.apply(HasNotV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Dedup = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (DedupV.bind.apply(DedupV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.In = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (In.bind.apply(In, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Out = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Out.bind.apply(Out, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Both = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Both.bind.apply(Both, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Range = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (RangeV.bind.apply(RangeV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Limit = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (LimitV.bind.apply(LimitV, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.InE = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (InE.bind.apply(InE, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.OutE = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (OutE.bind.apply(OutE, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.BothE = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (BothE.bind.apply(BothE, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.ShortestPathTo = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (ShortestPath.bind.apply(ShortestPath, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Subgraph = function () {
        return new Subgraph(this.api, this);
    };
    V.prototype.Count = function () {
        return new Count(this.api, this);
    };
    V.prototype.Values = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Values.bind.apply(Values, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Keys = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Keys.bind.apply(Keys, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Sum = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Keys.bind.apply(Keys, [void 0, this.api, this].concat(params)))();
    };
    V.prototype.Sort = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (SortV.bind.apply(SortV, [void 0, this.api, this].concat(params)))();
    };
    return V;
}(Step));
exports.V = V;
var E = /** @class */ (function (_super) {
    __extends(E, _super);
    function E() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    E.prototype.name = function () { return "E"; };
    E.prototype.serialize = function (data) {
        return SerializationHelper.unmarshalArray(data, GraphEdge);
    };
    E.prototype.Has = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasE.bind.apply(HasE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.HasEither = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasEitherE.bind.apply(HasEitherE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.HasKey = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasKeyE.bind.apply(HasKeyE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.HasNot = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasNotE.bind.apply(HasNotE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Dedup = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (DedupE.bind.apply(DedupE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Range = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (RangeE.bind.apply(RangeE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Limit = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (LimitE.bind.apply(LimitE, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.InV = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (InV.bind.apply(InV, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.OutV = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (OutV.bind.apply(OutV, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.BothV = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (BothV.bind.apply(BothV, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Subgraph = function () {
        return new Subgraph(this.api, this);
    };
    E.prototype.Count = function () {
        return new Count(this.api, this);
    };
    E.prototype.Values = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Values.bind.apply(Values, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Keys = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Keys.bind.apply(Keys, [void 0, this.api, this].concat(params)))();
    };
    E.prototype.Sort = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (SortE.bind.apply(SortE, [void 0, this.api, this].concat(params)))();
    };
    return E;
}(Step));
exports.E = E;
var HasV = /** @class */ (function (_super) {
    __extends(HasV, _super);
    function HasV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasV;
}(MixinStep(V, "Has")));
var HasE = /** @class */ (function (_super) {
    __extends(HasE, _super);
    function HasE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasE;
}(MixinStep(E, "Has")));
var HasEitherV = /** @class */ (function (_super) {
    __extends(HasEitherV, _super);
    function HasEitherV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasEitherV;
}(MixinStep(V, "HasEither")));
var HasEitherE = /** @class */ (function (_super) {
    __extends(HasEitherE, _super);
    function HasEitherE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasEitherE;
}(MixinStep(E, "HasEither")));
var HasKeyV = /** @class */ (function (_super) {
    __extends(HasKeyV, _super);
    function HasKeyV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasKeyV;
}(MixinStep(V, "HasKey")));
var HasKeyE = /** @class */ (function (_super) {
    __extends(HasKeyE, _super);
    function HasKeyE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasKeyE;
}(MixinStep(E, "HasKey")));
var HasNotV = /** @class */ (function (_super) {
    __extends(HasNotV, _super);
    function HasNotV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasNotV;
}(MixinStep(V, "HasNot")));
var HasNotE = /** @class */ (function (_super) {
    __extends(HasNotE, _super);
    function HasNotE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasNotE;
}(MixinStep(E, "HasNot")));
var DedupV = /** @class */ (function (_super) {
    __extends(DedupV, _super);
    function DedupV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return DedupV;
}(MixinStep(V, "Dedup")));
var DedupE = /** @class */ (function (_super) {
    __extends(DedupE, _super);
    function DedupE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return DedupE;
}(MixinStep(E, "Dedup")));
var RangeV = /** @class */ (function (_super) {
    __extends(RangeV, _super);
    function RangeV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return RangeV;
}(MixinStep(V, "Range")));
var RangeE = /** @class */ (function (_super) {
    __extends(RangeE, _super);
    function RangeE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return RangeE;
}(MixinStep(E, "Range")));
var LimitV = /** @class */ (function (_super) {
    __extends(LimitV, _super);
    function LimitV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return LimitV;
}(MixinStep(V, "Limit")));
var LimitE = /** @class */ (function (_super) {
    __extends(LimitE, _super);
    function LimitE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return LimitE;
}(MixinStep(E, "Limit")));
var SortV = /** @class */ (function (_super) {
    __extends(SortV, _super);
    function SortV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return SortV;
}(MixinStep(V, "Sort")));
var SortE = /** @class */ (function (_super) {
    __extends(SortE, _super);
    function SortE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return SortE;
}(MixinStep(E, "Sort")));
var Out = /** @class */ (function (_super) {
    __extends(Out, _super);
    function Out() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Out.prototype.name = function () { return "Out"; };
    return Out;
}(V));
exports.Out = Out;
var In = /** @class */ (function (_super) {
    __extends(In, _super);
    function In() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    In.prototype.name = function () { return "In"; };
    return In;
}(V));
exports.In = In;
var Both = /** @class */ (function (_super) {
    __extends(Both, _super);
    function Both() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Both.prototype.name = function () { return "Both"; };
    return Both;
}(V));
exports.Both = Both;
var OutV = /** @class */ (function (_super) {
    __extends(OutV, _super);
    function OutV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    OutV.prototype.name = function () { return "OutV"; };
    return OutV;
}(V));
exports.OutV = OutV;
var InV = /** @class */ (function (_super) {
    __extends(InV, _super);
    function InV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    InV.prototype.name = function () { return "InV"; };
    return InV;
}(V));
exports.InV = InV;
var OutE = /** @class */ (function (_super) {
    __extends(OutE, _super);
    function OutE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    OutE.prototype.name = function () { return "OutE"; };
    return OutE;
}(E));
exports.OutE = OutE;
var InE = /** @class */ (function (_super) {
    __extends(InE, _super);
    function InE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    InE.prototype.name = function () { return "InE"; };
    return InE;
}(E));
exports.InE = InE;
var BothE = /** @class */ (function (_super) {
    __extends(BothE, _super);
    function BothE() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    BothE.prototype.name = function () { return "BothE"; };
    return BothE;
}(E));
exports.BothE = BothE;
var BothV = /** @class */ (function (_super) {
    __extends(BothV, _super);
    function BothV() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    BothV.prototype.name = function () { return "BothV"; };
    return BothV;
}(V));
exports.BothV = BothV;
var ShortestPath = /** @class */ (function (_super) {
    __extends(ShortestPath, _super);
    function ShortestPath() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    ShortestPath.prototype.name = function () { return "ShortestPathTo"; };
    ShortestPath.prototype.toString = function () {
        if (this.previous !== null) {
            var gremlin = this.previous.toString() + ".";
            return gremlin + callString.apply(void 0, [this.name()].concat(this.params));
        }
        else {
            return this.name();
        }
    };
    ShortestPath.prototype.serialize = function (data) {
        var items = [];
        for (var obj in data) {
            items.push(SerializationHelper.unmarshalArray(data[obj], GraphNode));
        }
        return items;
    };
    return ShortestPath;
}(Step));
exports.ShortestPath = ShortestPath;
var Predicate = /** @class */ (function () {
    function Predicate(name) {
        var params = [];
        for (var _i = 1; _i < arguments.length; _i++) {
            params[_i - 1] = arguments[_i];
        }
        this.name = name;
        this.params = params;
    }
    Predicate.prototype.toString = function () {
        return callString.apply(void 0, [this.name].concat(this.params));
    };
    return Predicate;
}());
function NE(param) {
    return new Predicate("NE", param);
}
exports.NE = NE;
function GT(param) {
    return new Predicate("GT", param);
}
exports.GT = GT;
function LT(param) {
    return new Predicate("LT", param);
}
exports.LT = LT;
function GTE(param) {
    return new Predicate("GTE", param);
}
exports.GTE = GTE;
function LTE(param) {
    return new Predicate("LTE", param);
}
exports.LTE = LTE;
function IPV4RANGE(param) {
    return new Predicate("IPV4RANGE", param);
}
exports.IPV4RANGE = IPV4RANGE;
function REGEX(param) {
    return new Predicate("REGEX", param);
}
exports.REGEX = REGEX;
function WITHIN() {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Predicate.bind.apply(Predicate, [void 0, "WITHIN"].concat(params)))();
}
exports.WITHIN = WITHIN;
function WITHOUT() {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Predicate.bind.apply(Predicate, [void 0, "WITHOUT"].concat(params)))();
}
exports.WITHOUT = WITHOUT;
function INSIDE(from, to) {
    return new Predicate("INSIDE", from, to);
}
exports.INSIDE = INSIDE;
function OUTSIDE(from, to) {
    return new Predicate("OUTSIDE", from, to);
}
exports.OUTSIDE = OUTSIDE;
function BETWEEN(from, to) {
    return new Predicate("BETWEEN", from, to);
}
exports.BETWEEN = BETWEEN;
exports.FOREVER = new Predicate("FOREVER");
exports.NOW = new Predicate("NOW");
exports.DESC = new Predicate("DESC");
exports.ASC = new Predicate("ASC");
var Nodes = /** @class */ (function (_super) {
    __extends(Nodes, _super);
    function Nodes() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Nodes.prototype.name = function () { return "Nodes"; };
    return Nodes;
}(V));
exports.Nodes = Nodes;
var Value = /** @class */ (function (_super) {
    __extends(Value, _super);
    function Value() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Value.prototype.serialize = function (data) {
        return data;
    };
    return Value;
}(Step));
exports.Value = Value;
var Count = /** @class */ (function (_super) {
    __extends(Count, _super);
    function Count() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Count.prototype.name = function () { return "Count"; };
    return Count;
}(Value));
exports.Count = Count;
var Values = /** @class */ (function (_super) {
    __extends(Values, _super);
    function Values() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Values.prototype.name = function () { return "Values"; };
    return Values;
}(Value));
exports.Values = Values;
var Keys = /** @class */ (function (_super) {
    __extends(Keys, _super);
    function Keys() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Keys.prototype.name = function () { return "Keys"; };
    return Keys;
}(Value));
exports.Keys = Keys;
var Sum = /** @class */ (function (_super) {
    __extends(Sum, _super);
    function Sum() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Sum.prototype.name = function () { return "Sum"; };
    return Sum;
}(Value));
exports.Sum = Sum;
var Client = /** @class */ (function () {
    function Client(options) {
        options = options || {};
        this.baseURL = options["baseURL"] || "";
        this.username = options["username"] || "";
        this.password = options["password"] || "";
        this.cookie = options["cookie"] || "";
        this.alerts = new API(this, "alert", Alert);
        this.workflows = new API(this, "workflow", Workflow);
        this.gremlin = new GremlinAPI(this);
        this.G = this.gremlin.G();
    }
    Client.prototype.login = function () {
        var self = this;
        if (this.username && this.password) {
            return makeRequest(this, "/login", "POST", querystring.stringify({ "username": this.username, "password": this.password }), { "contentType": "application/x-www-form-urlencoded",
                "dataType": "" });
        }
        return Promise.resolve();
    };
    Client.prototype.request = function (url, method, data, opts) {
        if (this.cookie) {
            opts["headers"] = { "Cookie": this.cookie };
        }
        return makeRequest(this, url, method, data, opts);
    };
    return Client;
}());
exports.Client = Client;
