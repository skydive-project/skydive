import logging
import subprocess
import time
import unittest

from skydive.graph import Node, Edge
from skydive.rest.client import RESTClient
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDebugProtocol
from skydive.websocket.client import WSMessage
from skydive.websocket.client import NodeAddedMsgType, EdgeAddedMsgType


class WSTestClient(WSClientDebugProtocol):
    def onOpen(self):
        self.factory.kwargs["test"](self)
        self.stop_when_complete()


class SkydiveWSTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        logging.basicConfig(level=logging.DEBUG)
        subprocess.call(["docker", "run", "--name",
                         "skydive-docker-python-tests", "-p", "8082:8082",
                         "-d", "skydive/skydive", "analyzer"])
        time.sleep(10)

    @classmethod
    def tearDownClass(cls):
        subprocess.call(["docker", "rm", "-f", "skydive-docker-python-tests"])

    def test_connection(self):
        self.connected = False

        def is_connected(protocol):
            self.connected = True

        self.wsclient = WSClient("host-test",
                                 "ws://localhost:8082/ws/publisher",
                                 protocol=WSTestClient, test=is_connected)
        self.wsclient.connect()
        self.wsclient.start()

        self.assertEqual(self.connected, True, "failed to connect")

    def test_injection(self):
        def create_node(protocol):
            node = Node("TOR_TEST", "",
                        metadata={"Name": "Test TOR", "Type": "fabric"})
            msg = WSMessage("Graph", NodeAddedMsgType, node)
            protocol.sendWSMessage(msg)

            node = Node("PORT_TEST", "",
                        metadata={"Name": "Test port", "Type": "fabric"})
            msg = WSMessage("Graph", NodeAddedMsgType, node)
            protocol.sendWSMessage(msg)

            edge = Edge("TOR_L2LINK", "",
                        "TOR_TEST", "PORT_TEST",
                        metadata={"RelationType": "layer2"})
            msg = WSMessage("Graph", EdgeAddedMsgType, edge)
            protocol.sendWSMessage(msg)

        self.wsclient = WSClient("host-test2",
                                 "ws://localhost:8082/ws/publisher",
                                 protocol=WSTestClient, test=create_node)
        self.wsclient.connect()
        self.wsclient.start()

        time.sleep(1)

        restclient = RESTClient("localhost:8082")
        nodes = restclient.lookup_nodes("G.V().Has('Name', 'Test port')")
        self.assertEqual(len(nodes), 1, "should find one an only one node")

        tor_id = nodes[0].id
        self.assertEqual(tor_id, nodes[0].id, "wrong id for node")

        edges = restclient.lookup_edges(
            "G.E().Has('RelationType', 'layer2')")
        self.assertEqual(len(edges), 1, "should find one an only one edge")
