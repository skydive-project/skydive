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
var graffiti = require("../graffiti/js/api");
var Capture = /** @class */ (function (_super) {
    __extends(Capture, _super);
    function Capture() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return Capture;
}(graffiti.APIObject));
exports.Capture = Capture;
var PacketInjection = /** @class */ (function (_super) {
    __extends(PacketInjection, _super);
    function PacketInjection() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return PacketInjection;
}(graffiti.APIObject));
exports.PacketInjection = PacketInjection;
var Metric = /** @class */ (function () {
    function Metric() {
    }
    return Metric;
}());
exports.Metric = Metric;
var FlowLayer = /** @class */ (function () {
    function FlowLayer() {
    }
    return FlowLayer;
}());
exports.FlowLayer = FlowLayer;
var ICMPLayer = /** @class */ (function () {
    function ICMPLayer() {
    }
    return ICMPLayer;
}());
exports.ICMPLayer = ICMPLayer;
var FlowMetric = /** @class */ (function () {
    function FlowMetric() {
    }
    return FlowMetric;
}());
exports.FlowMetric = FlowMetric;
var RawPacket = /** @class */ (function () {
    function RawPacket() {
    }
    return RawPacket;
}());
exports.RawPacket = RawPacket;
var TCPMetric = /** @class */ (function () {
    function TCPMetric() {
    }
    return TCPMetric;
}());
exports.TCPMetric = TCPMetric;
var Flow = /** @class */ (function () {
    function Flow() {
    }
    return Flow;
}());
exports.Flow = Flow;
var Hops = /** @class */ (function (_super) {
    __extends(Hops, _super);
    function Hops() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Hops.prototype.name = function () { return "Hops"; };
    return Hops;
}(graffiti.V));
exports.Hops = Hops;
var CaptureNode = /** @class */ (function (_super) {
    __extends(CaptureNode, _super);
    function CaptureNode() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    CaptureNode.prototype.name = function () { return "CaptureNode"; };
    return CaptureNode;
}(graffiti.V));
exports.CaptureNode = CaptureNode;
var Metrics = /** @class */ (function (_super) {
    __extends(Metrics, _super);
    function Metrics() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Metrics.prototype.name = function () { return "Metrics"; };
    Metrics.prototype.serialize = function (data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Metric);
    };
    Metrics.prototype.Sum = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Sum).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Metrics.prototype.Aggregates = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Aggregates.bind.apply(Aggregates, [void 0, this.api, this].concat(params)))();
    };
    Metrics.prototype.Count = function () {
        return new graffiti.Count(this.api, this);
    };
    return Metrics;
}(graffiti.Step));
exports.Metrics = Metrics;
var Aggregates = /** @class */ (function (_super) {
    __extends(Aggregates, _super);
    function Aggregates() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Aggregates.prototype.name = function () { return "Aggregates"; };
    return Aggregates;
}(Metrics));
exports.Aggregates = Aggregates;
var RawPackets = /** @class */ (function (_super) {
    __extends(RawPackets, _super);
    function RawPackets() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    RawPackets.prototype.name = function () { return "RawPackets"; };
    RawPackets.prototype.serialize = function (data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], RawPacket);
    };
    RawPackets.prototype.BPF = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (BPF.bind.apply(BPF, [void 0, this.api, this].concat(params)))();
    };
    return RawPackets;
}(graffiti.Step));
exports.RawPackets = RawPackets;
var BPF = /** @class */ (function (_super) {
    __extends(BPF, _super);
    function BPF() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    BPF.prototype.name = function () { return "BPF"; };
    return BPF;
}(RawPackets));
exports.BPF = BPF;
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
var Socket = /** @class */ (function () {
    function Socket() {
    }
    return Socket;
}());
var Sockets = /** @class */ (function (_super) {
    __extends(Sockets, _super);
    function Sockets() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Sockets.prototype.name = function () { return "Sockets"; };
    Sockets.prototype.serialize = function (data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Socket);
    };
    Sockets.prototype.Values = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Values).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Sockets.prototype.Has = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasSockets.bind.apply(HasSockets, [void 0, this.api, this].concat(params)))();
    };
    return Sockets;
}(graffiti.Step));
exports.Sockets = Sockets;
var HasSockets = /** @class */ (function (_super) {
    __extends(HasSockets, _super);
    function HasSockets() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasSockets;
}(MixinStep(Sockets, "Has")));
var Flows = /** @class */ (function (_super) {
    __extends(Flows, _super);
    function Flows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    Flows.prototype.name = function () { return "Flows"; };
    Flows.prototype.serialize = function (data) {
        return graffiti.SerializationHelper.unmarshalArray(data, Flow);
    };
    Flows.prototype.Has = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasFlows.bind.apply(HasFlows, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.HasEither = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (HasEitherFlows.bind.apply(HasEitherFlows, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Count = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new graffiti.Count(this.api, this);
    };
    Flows.prototype.Values = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Values).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Keys = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Keys).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Sum = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Keys).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Sort = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (SortFlows.bind.apply(SortFlows, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Out = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Out).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.In = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.In).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Both = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Both).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Nodes = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        var _a;
        return new ((_a = graffiti.Nodes).bind.apply(_a, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Hops = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Hops.bind.apply(Hops, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.CaptureNode = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (CaptureNode.bind.apply(CaptureNode, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Metrics = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Metrics.bind.apply(Metrics, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.RawPackets = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (RawPackets.bind.apply(RawPackets, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Sockets = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (Sockets.bind.apply(Sockets, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Dedup = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (DedupFlows.bind.apply(DedupFlows, [void 0, this.api, this].concat(params)))();
    };
    Flows.prototype.Group = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (GroupFlows.bind.apply(GroupFlows, [void 0, this.api, this].concat(params)))();
    };
    return Flows;
}(graffiti.Step));
exports.Flows = Flows;
var HasFlows = /** @class */ (function (_super) {
    __extends(HasFlows, _super);
    function HasFlows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasFlows;
}(MixinStep(Flows, "Has")));
var HasEitherFlows = /** @class */ (function (_super) {
    __extends(HasEitherFlows, _super);
    function HasEitherFlows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return HasEitherFlows;
}(MixinStep(Flows, "HasEither")));
var SortFlows = /** @class */ (function (_super) {
    __extends(SortFlows, _super);
    function SortFlows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return SortFlows;
}(MixinStep(Flows, "Sort")));
var DedupFlows = /** @class */ (function (_super) {
    __extends(DedupFlows, _super);
    function DedupFlows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return DedupFlows;
}(MixinStep(Flows, "Dedup")));
var GroupFlows = /** @class */ (function (_super) {
    __extends(GroupFlows, _super);
    function GroupFlows() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    GroupFlows.prototype.name = function () { return "Group"; };
    GroupFlows.prototype.serialize = function (data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Flow);
    };
    GroupFlows.prototype.MoreThan = function () {
        var params = [];
        for (var _i = 0; _i < arguments.length; _i++) {
            params[_i] = arguments[_i];
        }
        return new (GroupFlows.bind.apply(GroupFlows, [void 0, this.api, this].concat(params)))();
    };
    return GroupFlows;
}(graffiti.Step));
exports.GroupFlows = GroupFlows;
graffiti.G.prototype.Flows = function () {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Flows.bind.apply(Flows, [void 0, this.api, this].concat(params)))();
};
graffiti.V.prototype.Metrics = function () {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Metrics.bind.apply(Metrics, [void 0, this.api, this].concat(params)))();
};
graffiti.V.prototype.Sockets = function () {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Sockets.bind.apply(Sockets, [void 0, this.api, this].concat(params)))();
};
graffiti.V.prototype.Flows = function () {
    var params = [];
    for (var _i = 0; _i < arguments.length; _i++) {
        params[_i] = arguments[_i];
    }
    return new (Flows.bind.apply(Flows, [void 0, this.api, this].concat(params)))();
};
var EdgeRule = /** @class */ (function (_super) {
    __extends(EdgeRule, _super);
    function EdgeRule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return EdgeRule;
}(graffiti.APIObject));
exports.EdgeRule = EdgeRule;
var NodeRule = /** @class */ (function (_super) {
    __extends(NodeRule, _super);
    function NodeRule() {
        return _super !== null && _super.apply(this, arguments) || this;
    }
    return NodeRule;
}(graffiti.APIObject));
exports.NodeRule = NodeRule;
var Client = /** @class */ (function (_super) {
    __extends(Client, _super);
    function Client(options) {
        var _this = _super.call(this, options) || this;
        _this.captures = new graffiti.API(_this, "capture", Capture);
        _this.edgeRules = new graffiti.API(_this, "edgerule", EdgeRule);
        _this.nodeRules = new graffiti.API(_this, "noderule", NodeRule);
        _this.packetInjections = new graffiti.API(_this, "injectpacket", PacketInjection);
        return _this;
    }
    return Client;
}(graffiti.Client));
