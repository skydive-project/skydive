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


class NodeRule(object):
    """
        Definition of a Skydive Node rule.
    """

    def __init__(self, uuid, action, metadata, name="",
                 description="", query=""):
        self.uuid = uuid
        self.name = name
        self.description = description
        self.metadata = metadata
        self.action = action
        self.query = query

    def repr_json(self):
        obj = {
            "UUID": self.uuid,
            "Action": self.action,
            "Metadata": self.metadata,
        }
        if self.name:
            obj["Name"] = self.name
        if self.description:
            obj["Description"] = self.description
        if self.query:
            obj["Query"] = self.query

    @classmethod
    def from_object(self, obj):
        return self(obj["UUID"], obj["Action"], obj["Metadata"],
                    name=obj.get("Name"),
                    description=obj.get("Description"),
                    query=obj.get("Query"))


class EdgeRule(object):
    """
        Definition of a Skydive Edge rule.
    """

    def __init__(self, uuid, src, dst, metadata, name="", description=""):
        self.uuid = uuid
        self.name = name
        self.description = description
        self.src = src
        self.dst = dst
        self.metadata = metadata

    def repr_json(self):
        obj = {
            "UUID": self.uuid,
            "Src": self.src,
            "Dst": self.dst,
            "Metadata": self.metadata,
        }
        if self.name:
            obj["Name"] = self.name
        if self.description:
            obj["Description"] = self.description

    @classmethod
    def from_object(self, obj):
        return self(obj["UUID"], obj["Src"], obj["Dst"], obj["Metadata"],
                    name=obj.get("Name"),
                    description=obj.get("Description"))

                    
                    
class InjectionRule(object):
    """
        Definition of a Skydive injection rule.
    """

    def __init__(self, uuid="", src="", dst="", srcip="", dstip="", srcmac="", dstmac="", 
                srcport="", dstport="", type="", payload="", trackingid="", icmpid="", 
                count="", interval="", increment="", starttime=""):
        self.uuid = uuid
        self.src = src
        self.dst = dst
        self.srcip = srcip
        self.dstip = dstip
        self.srcmac = srcmac
        self.dstmac = dstmac
        self.srcport = srcport
        self.dstport = dstport
        self.type = type
        self.payload = payload
        self.trackingid = trackingid
        self.icmpid = icmpid
        self.count = count
        self.interval = interval
        self.increment = increment
        self.starttime = starttime

    def repr_json(self):
        if self.uuid:
            obj["UUID"] = self.uuid 
        if self.src:
            obj["Src"] = self.src 
        if self.dst:
            obj["Dst"] = self.dst 
        if self.srcip:
            obj["SrcIP"] = self.srcip 
        if self.dstip:
            obj["DstIP"] = self.dstip 
        if self.srcmac:
            obj["SrcMAC"] = self.srcmac 
        if self.dstmac:
            obj["DstMAC"] = self.dstmac 
        if self.srcport:
            obj["SrcPort"] = self.srcport 
        if self.dstport:
            obj["DstPort"] = self.dstport 
        if self.type:
            obj["Type"] = self.type 
        if self.payload:
            obj["Payload"] = self.payload 
        if self.trackingid:
            obj["TrackingID"] = self.trackingid 
        if self.icmpid:
            obj["ICMPID"] = self.icmpid 
        if self.count:
            obj["Count"] = self.count 
        if self.interval:
            obj["Interval"] = self.interval 
        if self.increment:
            obj["Increment"] = self.increment 


    @classmethod
    def from_object(self, obj):
        return self(uuid=obj.get("UUID"),
                    src=obj.get("Src"),
                    dst=obj.get("Dst"),
                    srcip=obj.get("SrcIP"),
                    dstip=obj.get("DstIP"),
                    srcmac=obj.get("SrcMAC"),
                    dstmac=obj.get("DstMAC"),
                    srcport=obj.get("SrcPort"),
                    dstport=obj.get("DstPort"),
                    type=obj.get("Type"),
                    payload=obj.get("Payload"),
                    trackingid=obj.get("TrackingID"),
                    icmpid=obj.get("ICMPID"),
                    count=obj.get("Count"),
                    interval=obj.get("Interval"),
                    increment=obj.get("Increment"))
