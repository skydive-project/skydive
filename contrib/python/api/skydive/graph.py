#
# Copyright (C) 2017 Red Hat, Inc.
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


class GraphElement(object):
    """
        Definition of a Skydive graph element.
    """

    def __init__(self, id, host,
                 created_at=0, updated_at=0, deleted_at=0, revision=0,
                 metadata=None):
        self.id = id
        self.host = host
        self.metadata = metadata
        self.created_at = created_at
        self.updated_at = updated_at
        self.deleted_at = deleted_at
        self.revision = revision

    def repr_json(self):
        obj = {
            "ID": self.id,
            "Host": self.host,
            "Metadata": self.metadata
        }
        if self.created_at:
            obj["CreatedAt"] = self.created_at
        if self.updated_at:
            obj["UpdatedAt"] = self.updated_at
        if self.deleted_at:
            obj["DeletedAt"] = self.deleted_at
        return obj

    @classmethod
    def from_object(self, obj):
        return self(obj["ID"], obj["Host"],
                    created_at=obj.get("CreatedAt", 0),
                    updated_at=obj.get("UpdatedAt", 0),
                    deleted_at=obj.get("DeletedAt", 0),
                    revision=obj.get("Revision", 0),
                    metadata=obj.get("Metadata"))


class Node(GraphElement):
    """
        Definition of a Skydive graph Node, see GraphElement.
    """

    pass


class Edge(GraphElement):
    """
        Definition of a Skydive graph Edge, see GraphElement.
    """

    def __init__(self, id, host, parent, child,
                 created_at=0, updated_at=0, deleted_at=0, metadata=None):
        super(Edge, self).__init__(id, host,
                                   created_at=created_at,
                                   updated_at=updated_at,
                                   deleted_at=deleted_at,
                                   metadata=metadata)
        self.parent = parent
        self.child = child

    def repr_json(self):
        obj = super(Edge, self).repr_json()
        obj["Parent"] = self.parent
        obj["Child"] = self.child
        return obj

    @classmethod
    def from_object(self, obj):
        return self(obj["ID"], obj["Host"],
                    obj["Parent"], obj["Child"],
                    created_at=obj.get("CreatedAt", 0),
                    updated_at=obj.get("UpdatedAt", 0),
                    deleted_at=obj.get("DeletedAt", 0),
                    revision=obj.get("Revision", 0),
                    metadata=obj.get("Metadata"))
