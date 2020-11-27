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


class Alert(object):
    """
    Definition of a Skydive Alert.
    """

    def __init__(
        self, uuid, action, expression, trigger="graph", name="", description=""
    ):
        self.uuid = uuid
        self.name = name
        self.description = description
        self.expression = expression
        self.action = action
        self.trigger = trigger

    def repr_json(self):
        obj = {
            "UUID": self.uuid,
            "Action": self.action,
            "Expression": self.expression,
            "Trigger": self.trigger,
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
            obj["Action"],
            obj["Trigger"],
            obj["Expression"],
            name=obj.get("Name"),
            description=obj.get("Description"),
        )
