#
# Copyright (C) 2018 Red Hat, Inc.
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#  http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
#


class Capture(object):
    """
        Definition of a Skydive Capture
    """

    def __init__(self, uuid,  query,
                 name="", description="", count=0,
                 extra_tcp_metric=False, ip_defrag=False,
                 reassemble_tcp=False, layer_key_mode="L2"):
        self.uuid = uuid
        self.name = name
        self.description = description
        self.query = query
        self.count = count
        self.extra_tcp_metric = extra_tcp_metric
        self.ip_defrag = ip_defrag
        self.reassemble_tcp = reassemble_tcp
        self.layer_key_mode = layer_key_mode

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
        return obj

    @classmethod
    def from_object(self, obj):
        return self(obj["UUID"], obj["GremlinQuery"],
                    name=obj.get("Name"),
                    description=obj.get("Description"),
                    count=obj.get("Count"),
                    extra_tcp_metric=obj.get("ExtraTCPMetric"),
                    ip_defrag=obj.get("IPDefrag"),
                    reassemble_tcp=obj.get("ReassembleTCP"),
                    layer_key_mode=obj.get("LayerKeyMode")
                    )
