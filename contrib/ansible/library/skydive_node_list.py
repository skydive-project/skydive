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
module: skydive_node_list

short_description: Module to add nodes to Skydive topology

version_added: "1.0"

description:
    - "This module handles node adding to the Skydive topology"

options:
    analyzer:
        description:
            - analyzer address
        default: localhost:8082
        required: true
    ssl:
        description:
            - SSL enabled
        default: false
    insecure:
        description:
            - ignore SSL certification verification
        default: false
    username:
        description:
            - username authentication parameter
    password:
        description:
            - password authentication parameter
    nodes:
        description:
            - list of nodes
        required: true

notes:
  - The default value of seed may be unsufficient to disambiguate nodes.
  - Using list reduces the communication overhead for creating nodes.

author:
    - Sylvain Afchain (@safchain)
    - Pierre Crégut (@pierrecregut)
'''

EXAMPLES = '''
  - name: create tor
    skydive_node:
      nodes:
        - name: 'TOR1'
          type: "fabric"
          seed: "TOR1"
          metadata:
            Model: Cisco xxxx
        - name: 'TOR2'
          type: "fabric"
          seed: "TOR2"
          metadata:
            Model: Juniper yyyy
'''

RETURN = '''
UUID:
    description: The list of UUID of nodes generated
    type: list
'''

import os
import uuid

from ansible.module_utils.basic import AnsibleModule

from skydive.graph import Node

from skydive.websocket.client import NodeAddedMsgType
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDefaultProtocol
from skydive.websocket.client import WSMessage


class NodeInjectProtocol(WSClientDefaultProtocol):

    def onOpen(self):
        module = self.factory.kwargs["module"]
        params = self.factory.kwargs["params"]
        result = self.factory.kwargs["result"]

        if module.check_mode:
            self.stop()
            return

        try:
            host = params["host"]
            nodes = params["nodes"]
            uuids = []
            for node in nodes:
                metadata = dict()
                md = node.get("metadata", dict())
                for (k, v) in md.items():
                    metadata[k] = v
                metadata["Name"] = node["name"]
                metadata["Type"] = node["type"]
                seed = node.get("seed", "")
                if len(seed) == 0:
                    seed = "%s:%s" % (node["name"], node["type"])
                uid = str(uuid.uuid5(uuid.NAMESPACE_OID, seed))

                node = Node(str(uid), host, metadata=metadata)
                msg = WSMessage("Graph", NodeAddedMsgType, node)
                self.sendWSMessage(msg)
                uuids.append(uid)
            result["UUID"] = uuids
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
        nodes=dict(type='list', required=True),
        host=dict(type='str', default=""),
    )

    result = dict(
        changed=False
    )

    module = AnsibleModule(
        argument_spec=module_args,
        supports_check_mode=True
    )

    scheme = "ws"
    if module.params["ssl"]:
        scheme = "wss"

    try:
        url = "%s://%s/ws/publisher" % (scheme, module.params["analyzer"])
        wsclient = WSClient("ansible-" + str(os.getpid()) + "-"
                            + module.params["host"],
                            url,
                            protocol=NodeInjectProtocol, persistent=True,
                            insecure=module.params["insecure"],
                            username=module.params["username"],
                            password=module.params["password"],
                            module=module,
                            params=module.params,
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
