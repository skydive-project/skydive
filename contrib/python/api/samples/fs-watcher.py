import asyncio
import aionotify
import os
import stat
import hashlib
import argparse

from skydive.graph import Node, Edge
from skydive.websocket.client import WSClient
from skydive.websocket.client import WSClientDebugProtocol
from skydive.websocket.client import WSMessage
from skydive.websocket.client import NodeAddedMsgType, EdgeAddedMsgType


class WSClientInjectProtocol(WSClientDebugProtocol):

    def addNode(self, id, metadata):
        node = Node(hashlib.md5(id.encode()).hexdigest(),
                    "python-client", metadata=metadata)
        msg = WSMessage("Graph", NodeAddedMsgType, node)
        self.sendWSMessage(msg)

        return node

    def addEdge(self, id1, id2, metadata):
        id = id1 + id2
        edge = Edge(hashlib.md5(id.encode()).hexdigest(), "python-client",
                    id1, id2,
                    metadata=metadata)
        msg = WSMessage("Graph", EdgeAddedMsgType, edge)
        self.sendWSMessage(msg)

    async def inotify(self, watcher, root_node, path):
        loop = asyncio.get_event_loop()
        await watcher.setup(loop)

        while True:
            event = await watcher.get_event()

            file_path = "{}/{}".format(path, event.name)
            statinfo = os.lstat(file_path)

            type = "file"
            if stat.S_ISDIR(statinfo.st_mode):
                type = "folder"

            # create node for the current file
            node = self.addNode(file_path,
                                {"Name": event.name,
                                 "Type": type,
                                 "UID": statinfo.st_uid,
                                 "GID": statinfo.st_gid})

            # link the file with its parent node
            self.addEdge(root_node.id, node.id, {"RelationType": "ownership"})

            if stat.S_ISDIR(statinfo.st_mode):
                # if folder start another watcher
                sub_watcher = aionotify.Watcher()
                sub_watcher.watch(alias=event.name, path=file_path,
                                  flags=aionotify.Flags.CREATE)
                loop.create_task(self.inotify(sub_watcher, node, file_path))
            elif stat.S_ISLNK(statinfo.st_mode):
                # if link add an edge between two node
                src = os.readlink(file_path)
                self.addEdge(node.id, hashlib.md5(src.encode()).hexdigest(),
                             {"RelationType": "symlink", "Directed": True})

    def onOpen(self):
        folder = self.factory.kwargs["folder"]

        # create the host node
        host_node = self.addNode("HOST",
                                 {"Name": "Host", "Type": "host"})

        # create the root folder node
        root_node = self.addNode("ROOT",
                                 {"Name": folder, "Type": "root-folder"})

        # link host and root folder node
        self.addEdge(host_node.id, root_node.id, {"RelationType": "ownership"})

        # start watching the root folder
        watcher = aionotify.Watcher()
        watcher.watch(alias='root', path=folder,
                      flags=aionotify.Flags.CREATE)

        loop = asyncio.get_event_loop()
        loop.create_task(self.inotify(watcher, root_node, folder))

    def onClose(self, wasClean, code, reason):
        self.stop()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--folder', required=True)
    args = parser.parse_args()

    wsclient = WSClient("host-test", "ws://localhost:8082/ws/publisher",
                        protocol=WSClientInjectProtocol, persistent=True,
                        folder=args.folder)
    wsclient.connect()
    wsclient.start()


if __name__ == '__main__':
    main()
