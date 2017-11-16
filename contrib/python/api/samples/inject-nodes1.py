#
# create a fabric port node and link it to the TOR2 node
#

import sys

from skydive.websocket.client import Node, Edge
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDebugProtocol
from skydive.rest.client import RESTClient

from skydive.websocket.client import WSMessage

from skydive.websocket.client import SyncRequestMsgType, SyncReplyMsgType, \
    HostGraphDeletedMsgType, NodeUpdatedMsgType, NodeDeletedMsgType, \
    NodeAddedMsgType, EdgeUpdatedMsgType, EdgeDeletedMsgType, EdgeAddedMsgType


class WSClientInjectProtocol(WSClientDebugProtocol):

    def onOpen(self):
        print("WebSocket connection open.")

        # create a fabric port
        node = Node("PORT_TEST", "",
                    Name="Test port", Type="fabric")
        msg = WSMessage("Graph", NodeAddedMsgType, node).toJSON()
        self.sendMessage(msg)

        # get the TOR id
        restclient = RESTClient("localhost:8082")
        tor_id = restclient.get_node_id("G.V().Has('Name', 'TOR2')")

        # create a ownership + layer2 link
        edge = Edge(tor_id + node.id + "ownership", "",
                    tor_id, node.id,
                    RelationType="ownership")
        msg = WSMessage("Graph", EdgeAddedMsgType, edge).toJSON()
        self.sendMessage(msg)

        edge = Edge(tor_id + node.id + "layer2", "",
                    tor_id, node.id,
                    RelationType="layer2",
                    Type="fabric")
        msg = WSMessage("Graph", EdgeAddedMsgType, edge).toJSON()
        self.sendMessage(msg)

        self.sendClose()

        sys.exit(0)


def main():
    wsclient = WSClient("host-test", "ws://localhost:8082/ws",
                        protocol=WSClientInjectProtocol)
    wsclient.connect()

if __name__ == '__main__':
    main()
