#
# Copyright (C) 2017 Red Hat, Inc.
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

import json

try:
    import urllib.request as request
except ImportError:
    import urllib2 as request

from time import time
from socket import gethostname
from uuid import uuid4

from skydive.auth import Authenticate
from skydive.graph import Node, Edge
from skydive.rules import NodeRule, EdgeRule
from skydive.alerts import Alert
from skydive.captures import Capture
from skydive.packet_injector import PacketInjection
from skydive.tls import create_ssl_context


class BadRequest(Exception):
    pass


class RESTClient:
    INJECTION_PATH = "/api/injectpacket"

    def __init__(
        self,
        endpoint,
        scheme="http",
        username="",
        password="",
        cookies={},
        insecure=False,
        debug=0,
        cafile="",
        certfile="",
        keyfile="",
    ):
        self.endpoint = endpoint
        self.scheme = scheme
        self.username = username
        self.password = password
        self.insecure = insecure
        self.cafile = cafile
        self.certfile = certfile
        self.keyfile = keyfile
        self.debug = debug
        self.cookies = cookies

        self.auth = Authenticate(
            endpoint,
            scheme,
            username,
            password,
            cookies,
            insecure,
            cafile=cafile,
            certfile=certfile,
            keyfile=keyfile,
        )

    def request(self, path, method="GET", data=None):
        if self.username and not self.auth.authenticated:
            self.auth.login()

        handlers = []
        url = "%s://%s%s" % (self.scheme, self.endpoint, path)
        handlers.append(request.HTTPHandler(debuglevel=self.debug))
        handlers.append(request.HTTPCookieProcessor(self.auth.cookie_jar))

        if self.scheme == "https":
            context = create_ssl_context(
                self.insecure, self.cafile, self.certfile, self.keyfile
            )

            handlers.append(
                request.HTTPSHandler(debuglevel=self.debug, context=context)
            )

        if data is not None:
            encoded_data = data.encode()
        else:
            encoded_data = None

        opener = request.build_opener(*handlers)
        for k, v in self.cookies.items():
            opener.append = (k, v)

        headers = {"Content-Type": "application/json", "Accept": "application/json"}
        req = request.Request(url, data=encoded_data, headers=headers)
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
        data = json.dumps({"GremlinQuery": gremlin})

        objs = self.request("/api/topology", method="POST", data=data)

        if klass:
            return [klass.from_object(o) for o in objs]
        return objs

    def lookup_nodes(self, gremlin):
        return self.lookup(gremlin, Node)

    def lookup_edges(self, gremlin):
        return self.lookup(gremlin, Edge)

    def capture_create(
        self,
        query,
        name="",
        description="",
        extra_tcp_metric=False,
        ip_defrag=False,
        reassemble_tcp=False,
        layer_key_mode="L2",
        bpf_filter="",
        capture_type="",
        port=0,
        raw_pkt_limit=0,
        header_size=0,
        target="",
        target_type="",
        polling_interval=10,
        sampling_rate=1,
    ):
        data = {
            "GremlinQuery": query,
            "LayerKeyMode": layer_key_mode,
            "PollingInterval": polling_interval,
            "SamplingRate": sampling_rate,
        }

        if name:
            data["Name"] = name
        if description:
            data["Description"] = description
        if extra_tcp_metric:
            data["ExtraTCPMetric"] = True
        if ip_defrag:
            data["IPDefrag"] = True
        if reassemble_tcp:
            data["ReassembleTCP"] = True
        if bpf_filter:
            data["BPFFilter"] = bpf_filter
        if capture_type:
            data["Type"] = capture_type
        if raw_pkt_limit:
            data["RawPacketLimit"] = raw_pkt_limit
        if header_size:
            data["HeaderSize"] = header_size
        if port:
            data["Port"] = port
        if target:
            data["Target"] = target
        if target_type:
            data["TargetType"] = target_type

        c = self.request("/api/capture", method="POST", data=json.dumps(data))
        return Capture.from_object(c)

    def capture_list(self):
        objs = self.request("/api/capture")
        return [Capture.from_object(o) for o in objs.values()]

    def capture_delete(self, capture_id):
        path = "/api/capture/%s" % capture_id
        return self.request(path, method="DELETE")

    def alert_create(self, action, expression, trigger="graph"):
        data = json.dumps(
            {"Action": action, "Expression": expression, "Trigger": trigger}
        )
        a = self.request("/api/alert", method="POST", data=data)
        return Alert.from_object(a)

    def alert_list(self):
        objs = self.request("/api/alert")
        return [Alert.from_object(o) for o in objs.values()]

    def alert_delete(self, alert_id):
        path = "/api/alert/%s" % alert_id
        return self.request(path, method="DELETE")

    def noderule_create(self, action, metadata=None, query=""):
        data = json.dumps({"Action": action, "Metadata": metadata, "Query": query})
        r = self.request("/api/noderule", method="POST", data=data)
        return NodeRule.from_object(r)

    def noderule_list(self):
        objs = self.request("/api/noderule")
        return [NodeRule.from_object(o) for o in objs.values()]

    def noderule_delete(self, rule_id):
        path = "/api/noderule/%s" % rule_id
        return self.request(path, method="DELETE")

    def edgerule_create(self, src, dst, metadata):
        data = json.dumps({"Src": src, "Dst": dst, "Metadata": metadata})
        r = self.request("/api/edgerule", method="POST", data=data)
        return EdgeRule.from_object(r)

    def edgerule_list(self):
        objs = self.request("/api/edgerule")
        return [EdgeRule.from_object(o) for o in objs.values()]

    def edgerule_delete(self, rule_id):
        path = "/api/edgerule/%s" % rule_id
        self.request(path, method="DELETE")

    def node_create(self, node_id=None, host=None, metadata=None):
        now = int(time() * 1000)
        if not node_id:
            node_id = str(uuid4())

        if not host:
            host = gethostname()

        data = json.dumps(
            {
                "ID": node_id,
                "CreatedAt": now,
                "UpdatedAt": now,
                "Host": host,
                "Metadata": metadata,
                "Revision": 1,
            }
        )
        r = self.request("/api/node", method="POST", data=data)
        return Node.from_object(r)

    def node_update(self, node_id=None, patches=[]):
        data = json.dumps(patches)
        path = "/api/node/%s" % node_id
        r = self.request(path, method="PATCH", data=data)
        return Node.from_object(r)

    def node_list(self):
        objs = self.request("/api/node")
        return [Node.from_object(o) for o in objs.values()]

    def node_delete(self, node_id):
        path = "/api/node/%s" % node_id
        return self.request(path, method="DELETE")

    def edge_create(self, parent_id, child_id, edge_id=None, host=None, metadata=None):
        now = int(time() * 1000)
        if not edge_id:
            edge_id = str(uuid4())

        if not host:
            host = gethostname()

        data = json.dumps(
            {
                "ID": edge_id,
                "CreatedAt": now,
                "UpdatedAt": now,
                "Host": host,
                "Parent": parent_id,
                "Child": child_id,
                "Metadata": metadata,
            }
        )
        r = self.request("/api/edge", method="POST", data=data)
        return Edge.from_object(r)

    def edge_update(self, edge_id=None, patches=[]):
        data = json.dumps(patches)
        path = "/api/edge/%s" % edge_id
        r = self.request(path, method="PATCH", data=data)
        return Edge.from_object(r)

    def edge_list(self):
        objs = self.request("/api/edge")
        return [Edge.from_object(o) for o in objs.values()]

    def edge_delete(self, edge_id):
        path = "/api/edge/%s" % edge_id
        self.request(path, method="DELETE")

    def injection_create(
        self,
        src_query="",
        dst_query="",
        type="icmp4",
        payload="",
        interval=1000,
        src_ip="",
        dst_ip="",
        src_mac="",
        dst_mac="",
        count=1,
        icmp_id=0,
        src_port=0,
        dst_port=0,
        increment=False,
        ttl=64,
    ):

        data = json.dumps(
            {
                "Src": src_query,
                "Dst": dst_query,
                "SrcPort": src_port,
                "DstPort": dst_port,
                "SrcIP": src_ip,
                "DstIP": dst_ip,
                "SrcMAC": src_mac,
                "DstMAC": dst_mac,
                "Type": type,
                "Count": count,
                "ICMPID": icmp_id,
                "Interval": interval,
                "Payload": payload,
                "Increment": increment,
                "TTL": ttl,
            }
        )

        r = self.request(self.INJECTION_PATH, method="POST", data=data)
        return PacketInjection.from_object(r)

    def injection_delete(self, injection_id):
        path = self.INJECTION_PATH + "/" + injection_id
        return self.request(path, method="DELETE")

    def injection_list(self):
        objs = self.request(self.INJECTION_PATH)
        return [PacketInjection.from_object(o) for o in objs.values()]
