#
# create a fabric port node and link it to the TOR2 node
#

import logging
import sys

from skydive.graph import Node, Edge
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDebugProtocol
from skydive.rest.client import RESTClient

from skydive.websocket.client import WSMessage

from skydive.websocket.client import NodeAddedMsgType, EdgeAddedMsgType

LOG = logging.getLogger(__name__)


class WSClientInjectProtocol(WSClientDebugProtocol):

    def onOpen(self):
        print("WebSocket connection open.")

        # create a fabric port
        node = Node("PORT_TEST", "host-test",
                    metadata={"Name": "Test port !", "Type": "fabric"})
        msg = WSMessage("Graph", NodeAddedMsgType, node)
        self.sendWSMessage(msg)

        # get the TOR id
        restclient = RESTClient("localhost:8082")
        nodes = restclient.lookup_nodes("G.V().Has('Name', 'TOR2')")
        if len(nodes) != 1:
            print("More than one node found")
            sys.exit(-1)
        tor_id = nodes[0].id

        # create a ownership + layer2 link
        edge = Edge(tor_id + node.id + "ownership", "host-test",
                    tor_id, node.id,
                    metadata={"RelationType": "ownership"})
        msg = WSMessage("Graph", EdgeAddedMsgType, edge)
        self.sendWSMessage(msg)

        edge = Edge(tor_id + node.id + "layer2", "",
                    tor_id, node.id,
                    metadata={"RelationType": "layer2", "Type": "fabric"})
        msg = WSMessage("Graph", EdgeAddedMsgType, edge)
        self.sendWSMessage(msg)

        print("Success!")
        self.sendClose()

    def onClose(self, wasClean, code, reason):
        self.stop()


def main():
    logging.basicConfig(level=logging.INFO)
    wsclient = WSClient("host-test", "ws://localhost:8082/ws/publisher",
                        protocol=WSClientInjectProtocol, persistent=True)
    wsclient.connect()
    wsclient.start()


if __name__ == '__main__':
    main()
