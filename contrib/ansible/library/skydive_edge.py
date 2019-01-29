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
    name:
        description:
            - name of the node
        required: true
    relation_type:
        description:
            - "relation type of the node, ex: ownership, layer2, layer3"
        required: true
    node1:
        description:
            - first node of the link, can be either an ID or a gremlin expression
        required: true
    node2:
        description:
            - second node of the link, can be either an ID or a gremlin expression
        required: true
    host:
        description:
            - host of the node

author:
    - Sylvain Afchain (@safchain)
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
    skydive_edge:
      node1: "{{ tor_result.UUID }}"
      node2: "{{ port1_result.UUID }}"
      relation_type: ownership
  - name: link tor and port 2
    skydive_edge:
      node1: "{{ tor_result.UUID }}"
      node2: "{{ port2_result.UUID }}"
      relation_type: ownership
  - name: link port 1 and eth0
    skydive_edge:
      node1: "{{ port1_result.UUID }}"
      node2: G.V().Has('Name', 'tun0')
      relation_type: layer2
'''

RETURN = '''
original_message:
    description: The original name param that was passed in
    type: str
message:
    description: The output message that the sample module generates
'''

import os
import uuid

from ansible.module_utils.basic import AnsibleModule

from skydive.graph import Node, Edge
from skydive.rest.client import RESTClient

from skydive.websocket.client import NodeAddedMsgType, EdgeAddedMsgType
from skydive.websocket.client import WSClient, WSClientDefaultProtocol, WSMessage


class EdgeInjectProtocol(WSClientDefaultProtocol):

    def onOpen(self):
        module = self.factory.kwargs["module"]
        params = self.factory.kwargs["params"]
        result = self.factory.kwargs["result"]

        node1 = self.factory.kwargs["node1"]
        node2 = self.factory.kwargs["node2"]

        if module.check_mode:
            self.stop()
            return

        try:
            host = params["host"]

            metadata = params["metadata"]
            metadata["RelationType"] = params["relation_type"]

            uid = uuid.uuid5(uuid.NAMESPACE_OID, "%s:%s:%s" %
                             (node1, node2, params["relation_type"]))

            edge = Edge(str(uid), host,
                        node1, node2, metadata=metadata)

            msg = WSMessage("Graph", EdgeAddedMsgType, edge)
            self.sendWSMessage(msg)

            result["UUID"] = str(uid)
        except Exception as e:
            module.fail_json(
                msg='Error during topology update %s' % e, **result)
        finally:
            self.stop()


def get_node_id(module, node_selector):
    scheme = "http"
    if module.params["ssl"]:
        scheme = "https"

    if node_selector.startswith("G.") or node_selector.startswith("g."):
        restclient = RESTClient(module.params["analyzer"], scheme=scheme,
                                insecure=module.params["insecure"],
                                username=module.params["username"],
                                password=module.params["password"])
        nodes = restclient.lookup_nodes(node_selector)
        if len(nodes) == 0:
            raise Exception("Node not found: %s" % node_selector)
        elif len(nodes) > 1:
            raise Exception(
                "Node selection should return only one node: %s" % node_selector)

        return str(nodes[0].id)

    return node_selector


def run_module():
    module_args = dict(
        analyzer=dict(type='str', default="127.0.0.1:8082"),
        ssl=dict(type='bool', default=False),
        insecure=dict(type='bool', default=False),
        username=dict(type='str', default=""),
        password=dict(type='str', default="", no_log=True),
        relation_type=dict(type='str', required=True),
        node1=dict(type='str', required=True),
        node2=dict(type='str', required=True),
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
        node1 = get_node_id(module, module.params["node1"])
        node2 = get_node_id(module, module.params["node2"])
    except Exception as e:
        module.fail_json(
            msg='Error during topology request %s' % e, **result)

    scheme = "ws"
    if module.params["ssl"]:
        scheme = "wss"

    try:
        wsclient = WSClient("ansible-" + str(os.getpid()) + "-"
                            + module.params["host"],
                            "%s://%s/ws/publisher" % (scheme,
                                                      module.params["analyzer"]),
                            protocol=EdgeInjectProtocol, persistent=True,
                            insecure=module.params["insecure"],
                            username=module.params["username"],
                            password=module.params["password"],
                            module=module,
                            params=module.params,
                            node1=node1,
                            node2=node2,
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
