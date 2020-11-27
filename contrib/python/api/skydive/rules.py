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


class NodeRule(object):
    """
    Definition of a Skydive Node rule.
    """

    def __init__(self, uuid, action, metadata, name="", description="", query=""):
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
        return obj

    @classmethod
    def from_object(self, obj):
        return self(
            obj["UUID"],
            obj["Action"],
            obj["Metadata"],
            name=obj.get("Name"),
            description=obj.get("Description"),
            query=obj.get("Query"),
        )


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
        return obj

    @classmethod
    def from_object(self, obj):
        return self(
            obj["UUID"],
            obj["Src"],
            obj["Dst"],
            obj["Metadata"],
            name=obj.get("Name"),
            description=obj.get("Description"),
        )
