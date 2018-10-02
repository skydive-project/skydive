import logging
import subprocess
import time
import unittest
import os

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

        cls.schemeWS = "ws"
        cls.schemeHTTP = "http"
        if "SKYDIVE_PYTHON_TESTS_TLS" in os.environ:
            cls.schemeWS = "wss"
            cls.schemeHTTP = "https"

        cls.username = ""
        cls.password = ""
        cls.auth = False
        if "SKYDIVE_PYTHON_TESTS_USERPASS" in os.environ:
            cls.auth = True
            userpass = os.environ["SKYDIVE_PYTHON_TESTS_USERPASS"]
            cls.username, cls.password = userpass.split(":")

        extraArgs = []
        if "SKYDIVE_PYTHON_TESTS_MAPFILE" in os.environ:
            files = os.environ["SKYDIVE_PYTHON_TESTS_MAPFILE"]
            if files:
                for f in files.split(","):
                    extraArgs.append("-v")
                    extraArgs.append(f)

        subprocess.call(["docker", "run", "--name",
                         "skydive-docker-python-tests", "-p", "8082:8082"] +
                        extraArgs +
                        ["-d", "skydive/skydive:devel", "analyzer"])
        time.sleep(10)

    @classmethod
    def tearDownClass(cls):
        subprocess.call(["docker", "rm", "-f", "skydive-docker-python-tests"])

    def test_connection(self):
        self.connected = False

        def is_connected(protocol):
            self.connected = True

        self.wsclient = WSClient("host-test",
                                 self.schemeWS +
                                 "://localhost:8082/ws/publisher",
                                 protocol=WSTestClient, test=is_connected,
                                 username=self.username,
                                 password=self.password,
                                 insecure=True)
        self.wsclient.connect()
        if self.auth:
            ret = self.wsclient.login("localhost:8082", "toto")
            self.assertEqual(ret, False, "login() should failed")
            ret = self.wsclient.login("localhost:8082", "admin", "pass")
            self.assertEqual(ret, True, "login() failed")
            ret = self.wsclient.login()
            self.assertEqual(ret, True, "login() failed")
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

            node = Node("BAD_NODE", "",
                        metadata={"Name": "Bad node"})
            msg = WSMessage("Graph", NodeAddedMsgType, node)
            protocol.sendWSMessage(msg)

            node = Node("BAD_NETNS", "",
                        metadata={"Name": "Bad netns", "Type": "netns"})
            msg = WSMessage("Graph", NodeAddedMsgType, node)
            protocol.sendWSMessage(msg)

            edge = Edge("TOR_L2LINK", "",
                        "TOR_TEST", "PORT_TEST",
                        metadata={"RelationType": "layer2"})
            msg = WSMessage("Graph", EdgeAddedMsgType, edge)
            protocol.sendWSMessage(msg)

            edge = Edge("BAD_LINK", "",
                        "", "",
                        metadata={"RelationType": "layer2"})
            msg = WSMessage("Graph", EdgeAddedMsgType, edge)
            protocol.sendWSMessage(msg)

        self.wsclient = WSClient("host-test2",
                                 self.schemeWS +
                                 "://localhost:8082/ws/publisher",
                                 protocol=WSTestClient, test=create_node,
                                 username=self.username,
                                 password=self.password,
                                 insecure=True)
        self.wsclient.connect()
        self.wsclient.start()

        time.sleep(1)

        restclient = RESTClient("localhost:8082",
                                scheme=self.schemeHTTP,
                                username=self.username,
                                password=self.password,
                                insecure=True)
        nodes = restclient.lookup_nodes("G.V().Has('Name', 'Test port')")
        self.assertEqual(len(nodes), 1, "should find one an only one node")

        tor_id = nodes[0].id
        self.assertEqual(tor_id, nodes[0].id, "wrong id for node")

        nodes = restclient.lookup_nodes("G.V().Has('Name', 'Bad netns')")
        self.assertEqual(len(nodes), 0, "should find no 'Bad netns' node")

        nodes = restclient.lookup_nodes("G.V().Has('Name', 'Bad node')")
        self.assertEqual(len(nodes), 0, "should find no 'Bad node' node")

        edges = restclient.lookup_edges(
            "G.E().Has('RelationType', 'layer2')")
        self.assertEqual(len(edges), 1, "should find one an only one edge")
