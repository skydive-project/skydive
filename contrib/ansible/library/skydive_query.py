# -*- coding: utf-8 -*-
# Copyright (C) 2019 Orange.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ANSIBLE_METADATA = {
    'metadata_version': '1.1',
    'status': ['preview'],
    'supported_by': 'community'
}

DOCUMENTATION = '''
---
module: skydive_query

short_description: Module to query a Skydive instance

version_added: "1.0"

description:
    - "This module sends a gremlin query to a Skydive instance"

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
    query:
        description:
            - gremlin query
        required: true

author:
    - Sylvain Afchain (@safchain)
    - Pierre Crégut (@pierrecregut)
'''

EXAMPLES = '''
  - name: query host nodes registered in Skydive
    skydive_query:
      query: 'G.V().Has("Type","host")'
    register: result
'''

RETURN = '''
nodes:
    description: A list of JSON description of skydive nodes
    matching the query.
'''

from ansible.module_utils.basic import AnsibleModule
from skydive.rest.client import RESTClient


def make_query(params):
    """Performs the query with the rest client

       :param params: the parameters of the ansible module implemented.
       :returns: a list of nodes coded as dictionary (JSON representation
            of skydive node).
    """
    scheme = "https" if params["ssl"] else "http"
    restclient = RESTClient(params["analyzer"], scheme=scheme,
                            insecure=params["insecure"],
                            username=params["username"],
                            password=params["password"])
    nodes = restclient.lookup_nodes(params["query"])
    return [node.repr_json() for node in nodes]


def run_module():
    module_args = dict(
        analyzer=dict(type='str', default="127.0.0.1:8082"),
        ssl=dict(type='bool', default=False),
        insecure=dict(type='bool', default=False),
        username=dict(type='str', default=""),
        password=dict(type='str', default="", no_log=True),
        query=dict(type='str', required=True),
    )

    result = dict(
        # This module query the system but does not change its state
        changed=False
    )

    module = AnsibleModule(
        argument_spec=module_args,
        supports_check_mode=True
    )

    try:
        result["nodes"] = make_query(module.params)
    except Exception as e:
        module.fail_json(msg='Error during request %s' % e, **result)

    module.exit_json(**result)


def main():
    run_module()


if __name__ == '__main__':
    main()
