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

import * as graffiti from "../graffiti/js/api";

export class Capture extends graffiti.APIObject {
}

export class PacketInjection extends graffiti.APIObject {
}

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

export class Hops extends graffiti.V {
    name() { return "Hops" }
}

export class CaptureNode extends graffiti.V {
    name() { return "CaptureNode" }
}

export class Metrics extends graffiti.Step {
    name() { return "Metrics" }

    serialize(data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Metric);
    }

    Sum(...params: any[]): graffiti.Value {
        return new graffiti.Sum(this.api, this, ...params);
    }

    Aggregates(...params: any[]): Metrics {
        return new Aggregates(this.api, this, ...params);
    }

    Count(): graffiti.Value {
        return new graffiti.Count(this.api, this);
    }
}

export class Aggregates extends Metrics {
    name() { return "Aggregates" }
}

export class RawPackets extends graffiti.Step {
    name() { return "RawPackets" }

    serialize(data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], RawPacket);
    }

    BPF(...params: any[]): RawPackets {
        return new BPF(this.api, this, ...params);
    }
}

export class BPF extends RawPackets {
    name() { return "BPF" }
}

function MixinStep<T extends graffiti.Constructor<{}>>(Base: T, name: string) {
    return class extends Base {
        name() { return name }
        constructor(...args: any[]) {
            super(...args);
        }
    }
}

class Socket {
}

export class Sockets extends graffiti.Step {
    name() { return "Sockets" }

    serialize(data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Socket);
    }

    Values(...params: any[]): graffiti.Value {
        return new graffiti.Values(this.api, this, ...params);
    }

    Has(...params: any[]): Sockets {
        return new HasSockets(this.api, this, ...params);
    }
}

class HasSockets extends MixinStep(Sockets, "Has") { }

export class Flows extends graffiti.Step {
    name() { return "Flows" }

    serialize(data) {
        return graffiti.SerializationHelper.unmarshalArray(data, Flow);
    }

    Has(...params: any[]): Flows {
        return new HasFlows(this.api, this, ...params);
    }

    HasEither(...params: any[]): Flows {
        return new HasEitherFlows(this.api, this, ...params);
    }

    Count(...params: any[]): graffiti.Value {
        return new graffiti.Count(this.api, this);
    }

    Values(...params: any[]): graffiti.Value {
        return new graffiti.Values(this.api, this, ...params);
    }

    Keys(...params: any[]): graffiti.Value {
        return new graffiti.Keys(this.api, this, ...params);
    }

    Sum(...params: any[]): graffiti.Value {
        return new graffiti.Keys(this.api, this, ...params);
    }

    Sort(...params: any[]): Flows {
        return new SortFlows(this.api, this, ...params);
    }

    Out(...params: any[]): graffiti.V {
        return new graffiti.Out(this.api, this, ...params);
    }

    In(...params: any[]): graffiti.V {
        return new graffiti.In(this.api, this, ...params);
    }

    Both(...params: any[]): graffiti.V {
        return new graffiti.Both(this.api, this, ...params);
    }

    Nodes(...params: any[]): graffiti.V {
        return new graffiti.Nodes(this.api, this, ...params);
    }

    Hops(...params: any[]): graffiti.V {
        return new Hops(this.api, this, ...params);
    }

    CaptureNode(...params: any[]): graffiti.V {
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

    Group(...params: any[]): GroupFlows {
        return new GroupFlows(this.api, this, ...params);
    }
}

class HasFlows extends MixinStep(Flows, "Has") { }
class HasEitherFlows extends MixinStep(Flows, "HasEither") { }
class SortFlows extends MixinStep(Flows, "Sort") { }
class DedupFlows extends MixinStep(Flows, "Dedup") { }

export class GroupFlows extends graffiti.Step {
    name() { return "Group" }

    serialize(data) {
        return graffiti.SerializationHelper.unmarshalMapArray(data[0], Flow);
    }

    MoreThan(...params: any[]): GroupFlows {
        return new GroupFlows(this.api, this, ...params);
    }
}

declare module "../graffiti/js/api" {
    interface G {
        Flows(...params: any[]);
    }

    interface V {
        Metrics(...params: any[]);
        Sockets(...params: any[]);
        Flows(...params: any[]);
    }
}

graffiti.G.prototype.Flows = function(...params: any[]) {
    return new Flows(this.api, this, ...params);
}

graffiti.V.prototype.Metrics = function(...params: any[]): Metrics {
    return new Metrics(this.api, this, ...params);
}

graffiti.V.prototype.Sockets = function(...params: any[]): Sockets {
    return new Sockets(this.api, this, ...params);
}

graffiti.V.prototype.Flows = function(...params: any[]): Flows {
    return new Flows(this.api, this, ...params)
}

export class EdgeRule extends graffiti.APIObject {
}

export class NodeRule extends graffiti.APIObject {
}

class Client extends graffiti.Client {
    captures: graffiti.API<Capture>
    edgeRules: graffiti.API<EdgeRule>
    nodeRules: graffiti.API<NodeRule>
    packetInjections: graffiti.API<PacketInjection>

    constructor(options: Object) {
        super(options);

        this.captures = new graffiti.API(this, "capture", Capture);
        this.edgeRules = new graffiti.API(this, "edgerule", EdgeRule);
        this.nodeRules = new graffiti.API(this, "noderule", NodeRule);
        this.packetInjections = new graffiti.API(this, "injectpacket", PacketInjection);
    }
}
