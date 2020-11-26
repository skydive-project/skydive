#
# Copyright (C) 2018 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy ofthe License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specificlanguage governing permissions and
# limitations under the License.
#


class Capture(object):
    """
    Definition of a Skydive Capture
    """

    def __init__(
        self,
        uuid,
        query,
        name="",
        description="",
        count=0,
        extra_tcp_metric=False,
        ip_defrag=False,
        reassemble_tcp=False,
        layer_key_mode="L2",
        bpf_filter="",
        capture_type="",
        raw_pkt_limit=0,
        port=0,
        header_size=0,
        target="",
        target_type="",
        polling_interval=10,
        sampling_rate=1,
    ):
        self.uuid = uuid
        self.name = name
        self.description = description
        self.query = query
        self.count = count
        self.extra_tcp_metric = extra_tcp_metric
        self.ip_defrag = ip_defrag
        self.reassemble_tcp = reassemble_tcp
        self.layer_key_mode = layer_key_mode
        self.bpf_filter = bpf_filter
        self.capture_type = capture_type
        self.raw_pkt_limit = raw_pkt_limit
        self.port = port
        self.header_size = header_size
        self.target = target
        self.target_type = target_type
        self.polling_interval = polling_interval
        self.sampling_rate = sampling_rate

    def repr_json(self):
        obj = {
            "UUID": self.uuid,
            "GremlinQuery": self.query,
            "LayerKeyMode": self.layer_key_mode,
            "Count": self.count,
        }
        if self.name:
            obj["Name"] = self.name
        if self.description:
            obj["Description"] = self.description
        if self.extra_tcp_metric:
            obj["ExtraTCPMetric"] = True
        if self.ip_defrag:
            obj["IPDefrag"] = True
        if self.reassemble_tcp:
            obj["ReassembleTCP"] = True
        if self.bpf_filter:
            obj["BPFFilter"] = self.bpf_filter
        if self.capture_type:
            obj["Type"] = self.capture_type
        if self.raw_pkt_limit:
            obj["RawPacketLimit"] = self.raw_pkt_limit
        if self.header_size:
            obj["HeaderSize"] = self.header_size
        if self.port:
            obj["Port"] = self.port
        if self.target:
            obj["Target"] = self.target
        if self.target_type:
            obj["TargetType"] = self.target_type
        if self.polling_interval:
            obj["PollingInterval"] = self.polling_interval
        if self.sampling_rate:
            obj["SamplingRate"] = self.sampling_rate
        return obj

    @classmethod
    def from_object(self, obj):
        return self(
            obj["UUID"],
            obj["GremlinQuery"],
            name=obj.get("Name"),
            description=obj.get("Description"),
            count=obj.get("Count"),
            extra_tcp_metric=obj.get("ExtraTCPMetric"),
            ip_defrag=obj.get("IPDefrag"),
            reassemble_tcp=obj.get("ReassembleTCP"),
            layer_key_mode=obj.get("LayerKeyMode"),
            bpf_filter=obj.get("BPFFilter"),
            capture_type=obj.get("Type"),
            raw_pkt_limit=obj.get("RawPacketLimit"),
            header_size=obj.get("HeaderSize"),
            port=obj.get("Port"),
            target=obj.get("Target"),
            target_type=obj.get("TargetType"),
            polling_interval=obj.get("PollingInterval"),
            sampling_rate=obj.get("SamplingRate"),
        )
