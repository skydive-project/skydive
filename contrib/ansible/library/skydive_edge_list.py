# -*- coding: utf-8 -*-

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

ANSIBLE_METADATA = {
    'metadata_version': '1.1',
    'status': ['preview'],
    'supported_by': 'community'
}

DOCUMENTATION = '''
---
module: skydive_edge

short_description: Module to add edges to Skydive topology

version_added: "1.0"

description:
    - "This module handles edge adding to the Skydive topology"

options:
    analyzer:
        description:
            - analyzer address
        default: localhost:8082
        required: true
    ssl:
        description:
            - SSL enabled
    insecure:
        description:
            - ignore SSL certification verification
    username:
        description:
            - username authentication parameter
    password:
        description:
            - password authentication parameter
    edges:
        description:
            - list of the edges. Each edge must contain node1,
              node2 and relation_type fields
        required: true
    host:
        description:
            - host of the node

notes:
  - Using list reduces the communication overhead for creating edges.
  - Node definitions must be UUIDs. Gremlin queries require a communication
    roundtrip.

author:
    - Sylvain Afchain (@safchain)
    - Pierre Crégut (@pierrecregut)
'''

EXAMPLES = '''
  - name: create tor
    skydive_node:
      name: 'TOR'
      type: "fabric"
      metadata:
        Model: Cisco xxxx
    register: tor_result
  - name: create port 1
    skydive_node:
      name: 'PORT1'
      type: 'fabric'
    register: port1_result
  - name: create port 2
    skydive_node:
      name: 'PORT2'
      type: 'fabric'
    register: port2_result
  - name: link tor and port 1
    skydive_edge_list:
      edges:
        - node1: "{{ tor_result.UUID }}"
          node2: "{{ port1_result.UUID }}"
          relation_type: ownership
        - node1: "{{ tor_result.UUID }}"
          node2: "{{ port2_result.UUID }}"
          relation_type: ownership
'''

RETURN = '''
UUID:
    description: The list of UUID of nodes generated
    type: list
'''

import os
import uuid

from ansible.module_utils.basic import AnsibleModule

from skydive.graph import Edge

from skydive.websocket.client import EdgeAddedMsgType
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDefaultProtocol
from skydive.websocket.client import WSMessage


class EdgeInjectProtocol(WSClientDefaultProtocol):

    def onOpen(self):
        module = self.factory.kwargs["module"]
        params = self.factory.kwargs["params"]
        result = self.factory.kwargs["result"]
        edges = self.factory.kwargs["edges"]

        if module.check_mode:
            self.stop()
            return

        try:
            host = params["host"]
            uuids = []
            for edge in edges:
                metadata = edge.get("metadata", dict())
                metadata["RelationType"] = edge["relation_type"]
                node1 = edge["node1"]
                node2 = edge["node2"]
                uid = uuid.uuid5(uuid.NAMESPACE_OID, "%s:%s:%s" %
                                 (node1, node2, edge["relation_type"]))
                edge = Edge(str(uid), host, node1, node2, metadata=metadata)
                msg = WSMessage("Graph", EdgeAddedMsgType, edge)
                self.sendWSMessage(msg)
                uuids.append(uid)
            result["UUID"] = str(uuids)
        except Exception as e:
            module.fail_json(
                msg='Error during topology update %s' % e, **result)
        finally:
            self.factory.client.loop.call_soon(self.stop)


def run_module():
    module_args = dict(
        analyzer=dict(type='str', default="127.0.0.1:8082"),
        ssl=dict(type='bool', default=False),
        insecure=dict(type='bool', default=False),
        username=dict(type='str', default=""),
        password=dict(type='str', default="", no_log=True),
        edges=dict(type='list', required=True),
        host=dict(type='str', default=""),
        metadata=dict(type='dict', default=dict())
    )

    result = dict(
        changed=False
    )

    module = AnsibleModule(
        argument_spec=module_args,
        supports_check_mode=True
    )

    try:
        edges = module.params["edges"]
    except Exception as e:
        module.fail_json(
            msg='Error during topology request %s' % e, **result)

    scheme = "ws"
    if module.params["ssl"]:
        scheme = "wss"

    try:
        url = "%s://%s/ws/publisher" % (scheme, module.params["analyzer"])
        wsclient = WSClient("ansible-" + str(os.getpid()) + "-"
                            + module.params["host"],
                            url,
                            protocol=EdgeInjectProtocol, persistent=True,
                            insecure=module.params["insecure"],
                            username=module.params["username"],
                            password=module.params["password"],
                            module=module,
                            params=module.params,
                            edges=edges,
                            result=result)
        wsclient.connect()
        wsclient.start()
    except Exception as e:
        module.fail_json(msg='Connection error %s' % str(e), **result)

    result['changed'] = True

    module.exit_json(**result)


def main():
    run_module()


if __name__ == '__main__':
    main()
