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

import json
import ssl
try:
    import urllib.request as request
except ImportError:
    import urllib2 as request

from skydive.auth import Authenticate
from skydive.graph import Node, Edge


class BadRequest(Exception):
    pass


class RESTClient:
    def __init__(self, endpoint, scheme="http",
                 username="", password="",
                 insecure=False, debug=0):
        self.endpoint = endpoint
        self.scheme = scheme
        self.username = username
        self.password = password
        self.insecure = insecure
        self.debug = debug

        self.auth = Authenticate(endpoint, scheme,
                                 username, password, insecure)

    def request(self, path, method="GET", data=None):
        if self.username and not self.auth.authenticated:
            self.auth.login()

        handlers = []
        url = "%s://%s%s" % (self.scheme, self.endpoint, path)
        handlers.append(request.HTTPHandler(debuglevel=self.debug))
        handlers.append(request.HTTPCookieProcessor(self.auth.cookie_jar))

        if self.scheme == "https":
            if self.insecure:
                context = ssl._create_unverified_context()
            else:
                context = ssl._create_default_context()
            handlers.append(request.HTTPSHandler(debuglevel=self.debug,
                                                 context=context))

        if data is not None:
            encoded_data = data.encode()
        else:
            encoded_data = None

        opener = request.build_opener(*handlers)

        headers = {'Content-Type': 'application/json'}
        req = request.Request(url,
                              data=encoded_data,
                              headers=headers)
        req.get_method = lambda: method

        try:
            resp = opener.open(req)
        except request.HTTPError as e:
            self.auth.logout()
            raise BadRequest(e.read())

        data = resp.read()

        # DEPRECATED: workaround for skydive < 0.17
        # See PR #941
        if method == "DELETE":
            return data

        content_type = resp.headers.get("Content-type").split(";")[0]
        if content_type == "application/json":
            return json.loads(data.decode())
        return data

    def lookup(self, gremlin, klass=None):
        data = json.dumps(
            {"GremlinQuery": gremlin}
        )

        objs = self.request("/api/topology", method="POST", data=data)

        if klass:
            return [klass.from_object(o) for o in objs]
        return objs

    def lookup_nodes(self, gremlin):
        return self.lookup(gremlin, Node)

    def lookup_edges(self, gremlin):
        return self.lookup(gremlin, Edge)

    def capture_create(self, query):
        data = json.dumps(
            {"GremlinQuery": query}
        )
        return self.request("/api/capture", method="POST", data=data)

    def capture_list(self):
        return self.request("/api/capture")

    def capture_delete(self, capture_id):
        path = "/api/capture/%s" % capture_id
        return self.request(path, method="DELETE")
