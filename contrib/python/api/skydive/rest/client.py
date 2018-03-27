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
try:
    import urllib.request as request
except ImportError:
    import urllib2 as request

from skydive.graph import Node, Edge

import ssl


class RESTClient:
    def __init__(self, endpoint, ssl=False, insecure=False, **kwargs):
        self.endpoint = endpoint
        self.ssl = ssl
        self.insecure = insecure

        if "username" in kwargs:
            self.username = kwargs["username"]
        if "password" in kwargs:
            self.password = kwargs["password"]

    def lookup(self, gremlin, klass=None):
        data = json.dumps(
            {"GremlinQuery": gremlin}
        )

	ssl_handler = None

	scheme = "http"
        if self.ssl:
            scheme = "https"
            ctx = ssl.create_default_context()

            if self.insecure:
                ctx.check_hostname = False
                ctx.verify_mode = ssl.CERT_NONE

            ssl_handler = request.HTTPSHandler(context=ctx)

        url = "%s://%s/api/topology" % (scheme, self.endpoint)
        handler = request.HTTPHandler(debuglevel=1)
        if hasattr(self, "username"):
            mgr = request.HTTPPasswordMgrWithDefaultRealm()
            mgr.add_password(None, uri=url,
                             user=self.username,
                             passwd=self.password)
            handler = request.HTTPBasicAuthHandler(mgr)

        if ssl_handler:
            opener = request.build_opener(ssl_handler, handler)
        else:
            opener = request.build_opener(handler)

        req = request.Request(url,
                              data.encode(),
                              {'Content-Type': 'application/json'})

        resp = opener.open(req)
        if resp.getcode() != 200:
            return

        data = resp.read()
        objs = json.loads(data.decode())
        if klass:
            return [klass.from_object(o) for o in objs]
        return objs

    def lookup_nodes(self, gremlin):
        return self.lookup(gremlin, Node)

    def lookup_edges(self, gremlin):
        return self.lookup(gremlin, Edge)
