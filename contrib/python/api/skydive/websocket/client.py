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

try:
    import asyncio
except ImportError:
    import trollius as asyncio
import functools
import json
try:
    import http.client as httplib
except ImportError:
    import httplib
import logging
import ssl
import uuid
try:
    from urllib.parse import urlparse
except ImportError:
    from urlparse import urlparse
import warnings

from autobahn.asyncio.websocket import WebSocketClientProtocol
from autobahn.asyncio.websocket import WebSocketClientFactory

from skydive.auth import Authenticate
from skydive.encoder import JSONEncoder


LOG = logging.getLogger(__name__)

SyncRequestMsgType = "SyncRequest"
SyncReplyMsgType = "SyncReply"
OriginGraphDeletedMsgType = "OriginGraphDeleted"
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

    def debug_send(self, func, arg):
        LOG.debug("Running func %s", func.__name__)
        func(arg)

    def sendWSMessage(self, msg):
        self.factory.client.loop.call_soon(
            functools.partial(self.debug_send, self.sendMessage,
                              msg.to_json().encode()))

    def stop(self):
        self.factory.client.loop.stop()

    def stop_when_complete(self):
        self.factory.client.loop.call_soon(
            functools.partial(self.factory.client.loop.stop))


class WSClientDebugProtocol(WSClientDefaultProtocol):

    def onConnect(self, response):
        LOG.debug("Connected: %s", response.peer)

    def onMessage(self, payload, isBinary):
        if isBinary:
            LOG.debug("Binary message received: %d bytes", len(payload))
        else:
            LOG.debug("Text message received: %s", payload.decode('utf8'))

    def onOpen(self):
        LOG.debug("WebSocket connection opened.")

        if self.factory.client.sync:
            msg = WSMessage(
                "Graph", SyncRequestMsgType,
                SyncRequestMsg(self.factory.client.filter))
            self.sendWSMessage(msg)

    def onClose(self, wasClean, code, reason):
        LOG.debug("WebSocket connection closed: %s", reason)

    def sendWSMessage(self, msg):
        LOG.debug("Sending message: %s", msg.to_json())
        super(WSClientDebugProtocol, self).sendWSMessage(msg)


class WSClient(WebSocketClientProtocol):

    def __init__(self, host_id, endpoint,
                 protocol=WSClientDefaultProtocol,
                 username="", password="", cookie=None,
                 sync="", filter="", persistent=True,
                 insecure=False, type="skydive-python-client",
                 **kwargs):
        super(WSClient, self).__init__()
        self.host_id = host_id
        self.endpoint = endpoint
        self.username = username
        self.password = password
        if not cookie:
            self.cookies = None
        elif isinstance(cookie, list):
            self.cookies = cookie
        elif isinstance(cookie, dict):
            self.cookies = []
            for k, v in cookie.items():
                self.cookies.append("{}={}".format(k, v))
        else:
            self.cookies = [cookie, ]
        self.protocol = protocol
        self.type = type
        self.filter = filter
        self.persistent = persistent
        self.sync = sync
        self.insecure = insecure
        self.kwargs = kwargs

        self.url = urlparse(self.endpoint)

        scheme = "http"
        if self.url.scheme == "wss":
            scheme = "http"

        self.auth = Authenticate(
            "%s:%s" % (self.url.hostname, self.url.port),
            scheme=scheme, username=username, password=password,
            insecure=insecure)
        # We MUST initialize the loop here as the WebSocketClientFactory
        # needs it on init
        try:
            self.loop = asyncio.get_event_loop()
        except RuntimeError:
            self.loop = asyncio.new_event_loop()
            asyncio.set_event_loop(self.loop)

    def connect(self):
        factory = WebSocketClientFactory(self.endpoint)
        factory.protocol = self.protocol
        factory.client = self
        factory.kwargs = self.kwargs
        factory.headers["X-Host-ID"] = self.host_id
        factory.headers["X-Client-Type"] = self.type
        factory.headers["X-Client-Protocol"] = "json"
        if self.persistent:
            factory.headers["X-Persistence-Policy"] = "Persistent"
        else:
            factory.headers["X-Persistence-Policy"] = "DeleteOnDisconnect"

        if self.username:
            if self.auth.login():
                cookie = 'authtok={}'.format(self.auth.authtok)
                if self.cookies:
                    self.cookies.append(cookie)
                else:
                    self.cookies = [cookie, ]

        if self.filter:
            factory.headers["X-Gremlin-Filter"] = self.filter

        if self.cookies:
            factory.headers['Cookie'] = ';'.join(self.cookies)

        context = None
        if self.url.scheme == "wss":
            if self.insecure:
                context = ssl._create_unverified_context()
            else:
                context = ssl._create_default_context()

        coro = self.loop.create_connection(factory,
                                           self.url.hostname, self.url.port,
                                           ssl=context)
        (transport, protocol) = self.loop.run_until_complete(coro)
        LOG.debug('transport, protocol: %r, %r', transport, protocol)

    def login(self, host_spec="", username="", password=""):
        """ Authenticate with infrastructure via the Skydive analyzer

        This method will also set the authentication cookie to be used in
        the future requests
        :param host_spec: Host IP and port (e.g. 192.168.10.1:8082)
        :type host_spec: string
        :param username: Username to use for login
        :type username: string
        :param password: Password to use for login
        :type password: string
        :return: True on successful authentication, False otherwise
        """

        warnings.warn(
            "shouldn't use this function anymore ! use connect which handles"
            "handles authentication directly.",
            DeprecationWarning
        )

        scheme = "http"
        if not host_spec:
            u = urlparse(self.endpoint)
            host_spec = u.netloc
            if u.scheme == "wss":
                scheme = "https"
            if self.username:
                username = self.username
            if self.password:
                password = self.password

        auth = Authenticate(host_spec, scheme=scheme,
                            username=username, password=password)
        try:
            auth.login()
            cookie = 'authtok={}'.format(auth.authtok)
            if self.cookies:
                self.cookies.append(cookie)
            else:
                self.cookies = [cookie, ]
            return True
        except Exception:
            return False

    def start(self):
        try:
            self.loop.run_forever()
        except KeyboardInterrupt:
            self.loop.close()
        finally:
            pass

    def stop(self):
        self.loop.stop()
