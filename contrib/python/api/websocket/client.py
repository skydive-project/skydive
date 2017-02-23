import argparse
import json

try:
    import httplib
except:
    import http.client as httplib

import uuid

try:
    from urlparse import urlparse
except:
    from urllib.parse import urlparse

from autobahn.asyncio.websocket import WebSocketClientProtocol
from autobahn.asyncio.websocket import WebSocketClientFactory

SyncRequestMsgType = "SyncRequest"
SyncReplyMsgType = "SyncReply"
HostGraphDeletedMsgType = "HostGraphDeleted"
NodeUpdatedMsgType = "NodeUpdated"
NodeDeletedMsgType = "NodeDeleted"
NodeAddedMsgType = "NodeAdded"
EdgeUpdatedMsgType = "EdgeUpdated"
EdgeDeletedMsgType = "EdgeDeleted"
EdgeAddedMsgType = "EdgeAdded"

global args


class JSONEncoder(json.JSONEncoder):

    def default(self, obj):
        if hasattr(obj, 'reprJSON'):
            return obj.reprJSON()
        else:
            return json.JSONEncoder.default(self, obj)


class GraphElement(object):

    def __init__(self, id, host, **metadata):
        self.id = id
        self.host = host
        self.metadata = metadata

    def reprJSON(self):
        return {
            "ID": self.id,
            "Host": self.host,
            "Metadata": self.metadata
        }


class Node(GraphElement):
    pass


class Edge(GraphElement):

    def __init__(self, id, host, parent, child, **metadata):
        self.id = id
        self.host = host
        self.parent = parent
        self.child = child
        self.metadata = metadata

    def reprJSON(self):
        return {
            "ID": self.id,
            "Host": self.host,
            "Metadata": self.metadata,
            "Parent": self.parent,
            "Child": self.child
        }


class WSMessage(object):

    def __init__(self, ns, type, obj):
        self.uuid = uuid.uuid4().hex
        self.ns = ns
        self.type = type
        self.obj = obj
        self.status = httplib.OK

    def reprJSON(self):
        return {
            "UUID": self.uuid,
            "Namespace": self.ns,
            "Type": self.type,
            "Obj": self.obj,
            "Status": self.status
        }

    def toJSON(self):
        return json.dumps(self, cls=JSONEncoder)


class WSClientDefaultProtocol(WebSocketClientProtocol):

    def onConnect(self, response):
        print("Connected: {0}".format(response.peer))

    def onOpen(self):
        print("WebSocket connection open.")

    def onMessage(self, payload, isBinary):
        if isBinary:
            print("Binary message received: {0} bytes".format(len(payload)))
        else:
            print("Text message received: {0}".format(payload.decode('utf8')))

    def onClose(self, wasClean, code, reason):
        print("WebSocket connection closed: {0}".format(reason))
        self.transport.closeConnection()


class WSClientReplayProtocol(WSClientDefaultProtocol):

    def onOpen(self):
        print("WebSocket connection open.")

        print("Replaying: "+args.file)
        with open(args.file) as replay_file:
            data = json.load(replay_file)

            for node in data["Nodes"]:
                msg = WSMessage("Graph", NodeAddedMsgType, node).toJSON()
                self.sendMessage(msg)

            for edge in data["Edges"]:
                msg = WSMessage("Graph", EdgeAddedMsgType, edge).toJSON()
                self.sendMessage(msg)


class WSClient(WebSocketClientProtocol):

    def __init__(self, host_id, endpoint, type="",
                 protocol=WSClientDefaultProtocol):
        self.host_id = host_id
        self.endpoint = endpoint
        self.protocol = protocol
        self.type = type

    def connect(self):
        factory = WebSocketClientFactory(self.endpoint)
        factory.protocol = self.protocol
        factory.headers["X-Host-ID"] = self.host_id
        factory.headers["X-Client-Type"] = self.type

        loop = asyncio.get_event_loop()

        u = urlparse(self.endpoint)

        coro = loop.create_connection(factory, u.hostname, u.port)
        loop.run_until_complete(coro)

        try:
            loop.run_forever()
        except KeyboardInterrupt:
            pass
        finally:
            loop.close()


if __name__ == '__main__':
    try:
        import asyncio
    except ImportError:
        # Trollius >= 0.3 was renamed
        import trollius as asyncio

    parser = argparse.ArgumentParser()
    parser.add_argument('--analyzer', type=str, default="127.0.0.1:8082",
                        dest='analyzer',
                        help='address of the Skydive analyzer')

    subparsers = parser.add_subparsers(help='sub-command help', dest='mode')
    parser_replay = subparsers.add_parser('replay', help='replay help')
    parser_replay.add_argument('file', type=str, help='topology to replay')

    listen_replay = subparsers.add_parser('listen', help='listen help')
    args = parser.parse_args()

    if args.mode == "replay":
        protocol = WSClientReplayProtocol
    else:
        protocol = WSClientDefaultProtocol

    client = WSClient("Test", "ws://"+args.analyzer+"/ws",
                      protocol=protocol)
    client.connect()
