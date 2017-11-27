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

import argparse
import json
import os
import sys

from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDebugProtocol
from skydive.websocket.client import WSMessage

from skydive.websocket.client import NodeUpdatedMsgType, NodeDeletedMsgType, \
    NodeAddedMsgType, EdgeUpdatedMsgType, EdgeDeletedMsgType, EdgeAddedMsgType


class WSClientModifyProtocol(WSClientDebugProtocol):

    def onOpen(self):
        print("WebSocket connection open.")

        mode = self.factory.kwargs["mode"]
        file = self.factory.kwargs["file"]

        if mode and mode[-1] == 'e':
            print(mode[:-1] + "ing: " + file)
        else:
            print(mode + "ing: " + file)

        with open(file) as json_file:
            data = json.load(json_file)

            if "Nodes" in data:
                for node in data["Nodes"]:
                    if mode == 'add':
                        msg = WSMessage("Graph", NodeAddedMsgType, node)
                    elif mode == 'delete':
                        msg = WSMessage("Graph", NodeDeletedMsgType, node)
                    elif mode == 'update':
                        msg = WSMessage("Graph", NodeUpdatedMsgType, node)

                    self.sendWSMessage(msg)

            if "Edges" in data:
                for edge in data["Edges"]:
                    if mode == 'add':
                        msg = WSMessage("Graph", EdgeAddedMsgType, edge)
                    elif mode == 'delete':
                        msg = WSMessage("Graph", EdgeDeletedMsgType, edge)
                    elif mode == 'update':
                        msg = WSMessage("Graph", EdgeUpdatedMsgType, edge)

                    self.sendWSMessage(msg)

        self.stop()


def main():

    parser = argparse.ArgumentParser()
    parser.add_argument('--analyzer', type=str, default="127.0.0.1:8082",
                        dest='analyzer',
                        help='address of the Skydive analyzer')
    parser.add_argument('--host', type=str, default="Test",
                        dest='host',
                        help='client identifier')
    parser.add_argument('--username', type=str, default="",
                        dest='username',
                        help='client username')
    parser.add_argument('--password', type=str, default="",
                        dest='password',
                        help='client password')

    subparsers = parser.add_subparsers(
        help='sub-command help', dest='mode')
    parser_add = subparsers.add_parser(
        'add', help='add edges and nodes in the given json files')
    parser_add.add_argument('file', type=str, help='topology to add')

    parser_delete = subparsers.add_parser(
        'delete', help='delete edges and nodes in the given json files')
    parser_delete.add_argument('file', type=str, help='topology to delete')

    parser_update = subparsers.add_parser(
        'update', help='update edges and nodes in the given json files')
    parser_update.add_argument('file', type=str, help='topology to update')

    parser_listen = subparsers.add_parser('listen', help='listen help')
    parser_listen.add_argument(
        '--gremlin', type=str, default="",
        required=False, help='gremlin filter')
    parser_listen.add_argument(
        '--sync-request', default=False, required=False,
        action='store_true', help='send a request message')

    args = parser.parse_args()

    if not args.mode:
        parser.print_help()
        sys.exit(0)

    if args.mode == "listen":
        protocol = WSClientDebugProtocol
        endpoint = "/ws/subscriber"
        file = ""
        gremlin_filter = args.gremlin
        sync_request = args.sync_request
    else:
        protocol = WSClientModifyProtocol
        endpoint = "/ws/publisher"
        file = args.file
        gremlin_filter = ""
        sync_request = False
        if not os.path.isfile(args.file):
            raise ValueError("The file %s does not exist" % args.file)

    client = WSClient(args.host, "ws://" + args.analyzer + endpoint,
                      username=args.username,
                      password=args.password,
                      protocol=protocol,
                      filter=gremlin_filter,
                      sync=sync_request,
                      mode=args.mode,
                      file=file)
    client.connect()


if __name__ == '__main__':
    main()
