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
module: skydive_node

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
    name:
        description:
            - name of the node
        required: true
    type:
        description:
            - type of the node
        required: true
    host:
        description:
            - host of the node
    seed:
        description:
            - used to generate the UUID of the node
        default: <name>:<type>

notes:
  - The default value of seed may be unsufficient to disambiguate nodes.

author:
    - Sylvain Afchain (@safchain)
'''

EXAMPLES = '''
  - name: create tor
    skydive_node:
      name: 'TOR'
      type: "fabric"
      seed: "TOR1"
      metadata:
        Model: Cisco xxxx
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

            metadata = params["metadata"]
            metadata["Name"] = params["name"]
            metadata["Type"] = params["type"]

            seed = params["seed"]
            if len(seed) == 0:
                seed = "%s:%s" % (params["name"], params["type"])
            uid = str(uuid.uuid5(uuid.NAMESPACE_OID, seed))

            node = Node(str(uid), host, metadata=metadata)

            msg = WSMessage("Graph", NodeAddedMsgType, node)
            self.sendWSMessage(msg)

            result["UUID"] = uid
        except Exception as e:
            module.fail_json(
                msg='Error during topology update %s' % e, **result)
        finally:
            self.stop()


def run_module():
    module_args = dict(
        analyzer=dict(type='str', default="127.0.0.1:8082"),
        ssl=dict(type='bool', default=False),
        insecure=dict(type='bool', default=False),
        username=dict(type='str', default=""),
        password=dict(type='str', default="", no_log=True),
        name=dict(type='str', required=True),
        type=dict(type='str', required=True),
        host=dict(type='str', default=""),
        seed=dict(type='str', default=""),
        metadata=dict(type='dict', default=dict())
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
        wsclient = WSClient("ansible-" + str(os.getpid()) + "-"
                            + module.params["host"],
                            "%s://%s/ws/publisher" % (scheme,
                                                      module.params["analyzer"]),
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
