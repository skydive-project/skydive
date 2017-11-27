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

import asyncio
import base64
import json
import http.client as httplib
import uuid
from urllib.parse import urlparse

from autobahn.asyncio.websocket import WebSocketClientProtocol
from autobahn.asyncio.websocket import WebSocketClientFactory

from skydive.encoder import JSONEncoder


SyncRequestMsgType = "SyncRequest"
SyncReplyMsgType = "SyncReply"
HostGraphDeletedMsgType = "HostGraphDeleted"
NodeUpdatedMsgType = "NodeUpdated"
NodeDeletedMsgType = "NodeDeleted"
NodeAddedMsgType = "NodeAdded"
EdgeUpdatedMsgType = "EdgeUpdated"
EdgeDeletedMsgType = "EdgeDeleted"
EdgeAddedMsgType = "EdgeAdded"


class WSMessage(object):

    def __init__(self, ns, type, obj):
        self.uuid = uuid.uuid4().hex
        self.ns = ns
        self.type = type
        self.obj = obj
        self.status = httplib.OK

    def repr_json(self):
        return {
            "UUID": self.uuid,
            "Namespace": self.ns,
            "Type": self.type,
            "Obj": self.obj,
            "Status": self.status
        }

    def to_json(self):
        return json.dumps(self, cls=JSONEncoder)


class SyncRequestMsg:

    def __init__(self, filter):
        self.filter = filter

    def repr_json(self):
        return {
            "GremlinFilter": self.filter
        }

    def to_json(self):
        return json.dumps(self, cls=JSONEncoder)


class WSClientDefaultProtocol(WebSocketClientProtocol):

    def onClose(self, wasClean, code, reason):
        self.transport.closeConnection()

    def sendWSMessage(self, msg):
        self.sendMessage(msg.to_json().encode())

    def stop(self):
        self.factory.client.loop.stop()


class WSClientDebugProtocol(WSClientDefaultProtocol):

    def onConnect(self, response):
        print("Connected: {0}".format(response.peer))

    def onMessage(self, payload, isBinary):
        if isBinary:
            print("Binary message received: {0} bytes".format(len(payload)))
        else:
            print("Text message received: {0}".format(payload.decode('utf8')))

    def onOpen(self):
        print("WebSocket connection opened.")

        if self.factory.client.sync:
            msg = WSMessage(
                "Graph", SyncRequestMsgType,
                SyncRequestMsg(self.factory.client.filter))
            self.sendWSMessage(msg)

    def onClose(self, wasClean, code, reason):
        print("WebSocket connection closed: {0}".format(reason))


class WSClient(WebSocketClientProtocol):

    def __init__(self, host_id, endpoint, type="",
                 protocol=WSClientDefaultProtocol,
                 username="", password="", sync="", filter="",
                 **kwargs):
        self.host_id = host_id
        self.endpoint = endpoint
        self.username = username
        self.password = password
        self.protocol = protocol
        self.type = type
        self.filter = filter
        self.sync = sync
        self.kwargs = kwargs

    def connect(self):
        factory = WebSocketClientFactory(self.endpoint)
        factory.protocol = self.protocol
        factory.client = self
        factory.kwargs = self.kwargs
        factory.headers["X-Host-ID"] = self.host_id
        factory.headers["X-Client-Type"] = self.type

        if self.username:
            authorization = base64.b64encode(
                b"%s:%s" % (self.username, self.password)).decode("ascii")
            factory.headers["Authorization"] = 'Basic %s' % authorization

        if self.filter:
            factory.headers["X-Gremlin-Filter"] = self.filter

        self.loop = asyncio.get_event_loop()
        u = urlparse(self.endpoint)

        coro = self.loop.create_connection(factory, u.hostname, u.port)
        self.loop.run_until_complete(coro)

        try:
            self.loop.run_forever()
        except KeyboardInterrupt:
            pass
        finally:
            self.loop.close()
