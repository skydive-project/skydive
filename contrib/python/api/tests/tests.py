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
        cls.insecure = True
        cls.cafile = ""
        cls.certfile = ""
        cls.keyfile = ""

        if "SKYDIVE_PYTHON_TESTS_USERPASS" in os.environ:
            cls.auth = True
            userpass = os.environ["SKYDIVE_PYTHON_TESTS_USERPASS"]
            cls.username, cls.password = userpass.split(":")

        if "SKYDIVE_PYTHON_TESTS_CERTIFICATES" in os.environ:
            cls.insecure = False
            certificates = os.environ["SKYDIVE_PYTHON_TESTS_CERTIFICATES"]
            cls.cafile, cls.certfile, cls.keyfile = certificates.split(":")

    def new_rest_client(self):
        return RESTClient("localhost:8082",
                          scheme=self.schemeHTTP,
                          username=self.username,
                          password=self.password,
                          insecure=self.insecure,
                          cafile=self.cafile,
                          certfile=self.certfile,
                          keyfile=self.keyfile)

    def new_ws_client(self, id, endpoint, test):
        return WSClient(id,
                        self.schemeWS +
                        "://localhost:8082/ws/"+endpoint,
                        protocol=WSTestClient, test=test,
                        username=self.username,
                        password=self.password,
                        insecure=self.insecure,
                        cafile=self.cafile,
                        certfile=self.certfile,
                        keyfile=self.keyfile)

    def test_connection(self):
        self.connected = False

        def is_connected(protocol):
            self.connected = True

        self.wsclient = self.new_ws_client(id="host-test",
                                           endpoint="publisher",
                                           test=is_connected)
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

        self.wsclient = self.new_ws_client(id="host-test2",
                                           endpoint="publisher",
                                           test=create_node)
        self.wsclient.connect()
        self.wsclient.start()

        time.sleep(1)

        restclient = self.new_rest_client()
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

    def test_capture(self):
        restclient = self.new_rest_client()

        capture1 = restclient.capture_create(
            "G.V().Has('Name', 'test', 'Type', 'netns')")

        captures = restclient.capture_list()
        self.assertGreaterEqual(len(captures), 1, "no capture found")

        for capture in captures:
            if (capture.uuid == capture1.uuid):
                found = True
                break
        self.assertTrue(found, "created capture not found")

        restclient.capture_delete(capture1.uuid)

    def test_alert(self):
        restclient = self.new_rest_client()

        alert1 = restclient.alert_create(
            "https://localhost:8081",
            "G.V().Has('Name', 'alert-ns-webhook', 'Type', 'netns')")

        alerts = restclient.alert_list()
        self.assertGreaterEqual(len(alerts), 1, "no alerts found")
        for alert in alerts:
            if (alert.uuid == alert1.uuid):
                found = True
                break

        self.assertTrue(found, "created alert not found")

        restclient.alert_delete(alert1.uuid)

    def test_topology_rules(self):
        restclient = self.new_rest_client()

        noderule1 = restclient.noderule_create(
            "create", metadata={"Name": "node1", "Type": "fabric"})

        noderule2 = restclient.noderule_create(
            "create", metadata={"Name": "node2", "Type": "fabric"})

        time.sleep(1)

        edgerule1 = restclient.edgerule_create(
                "G.V().Has('Name', 'node1')", "G.V().Has('Name', 'node2')",
                {"RelationType": "layer2", "EdgeName": "my_edge"})

        time.sleep(1)

        node1 = restclient.lookup_nodes("G.V().Has('Name', 'node1')")
        self.assertEqual(len(node1), 1, "should find only one node as node1")

        node2 = restclient.lookup_nodes("G.V().Has('Name', 'node2')")
        self.assertEqual(len(node2), 1, "should find only one node as node2")

        edge = restclient.lookup_edges(
                "G.E().Has('RelationType', 'layer2', 'EdgeName', 'my_edge')")
        self.assertEqual(len(edge), 1, "should find only one edge")

        noderules = restclient.noderule_list()
        self.assertGreaterEqual(len(noderules), 2, "no noderules found")
        found = False
        for noderule in noderules:
            if (noderule.uuid == noderule1.uuid):
                found = True
                break

        self.assertTrue(found, "created noderule not found")

        edgerules = restclient.edgerule_list()
        self.assertGreaterEqual(len(edgerules), 1, "no edgerules found")
        found = False
        for edgerule in edgerules:
            if (edgerule.uuid == edgerule1.uuid):
                found = True
                break

        self.assertTrue(found, "created edgerule not found")
        restclient.edgerule_delete(edgerule1.uuid)
        restclient.noderule_delete(noderule1.uuid)
        restclient.noderule_delete(noderule2.uuid)

    def test_injections(self):
        restclient = self.new_rest_client()

        nodes = restclient.lookup("G.V().Has('Name', 'eth0')")

        testnode = nodes[0]["Metadata"]["TID"]

        query = "G.V().Has('TID', '" + testnode + "')"
        num_injections_before = len(restclient.injection_list())

        injection_response = restclient.injection_create(query, query,
                                                         count=1000)

        num_injections_after = len(restclient.injection_list())

        self.assertEqual(num_injections_after, num_injections_before + 1,
                         "injection creation didn't succeed")

        restclient.injection_delete(injection_response.uuid)

        num_injections_after_deletion = len(restclient.injection_list())

        self.assertEqual(num_injections_after_deletion, num_injections_before,
                         "injection deletion didn't succeed")
