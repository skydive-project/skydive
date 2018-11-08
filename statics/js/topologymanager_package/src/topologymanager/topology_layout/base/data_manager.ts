import { NodeRegistry } from './node/index';
import { EdgeRegistry } from './edge/index';
import { GroupRegistry } from './group/index';
import LayoutContext from './ui/layout_context';
import { Node } from './node/index';
import { Edge } from './edge/index';
import parseData, { parseSkydiveMessageWithOneNodeAndUpdateNode, parseSkydiveMessageWithOneNode, getNodeIDFromSkydiveMessageWithOneNode, getHostFromSkydiveMessageWithOneNode, getEdgeIDFromSkydiveMessageWithOneEdge, parseSkydiveMessageWithOneEdgeAndUpdateEdge, parseNewSkydiveEdgeAndUpdateDataManager } from './parsers/index';

export default class DataManager {
    nodeManager: NodeRegistry = new NodeRegistry();
    edgeManager: EdgeRegistry = new EdgeRegistry();
    groupManager: GroupRegistry = new GroupRegistry();
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
    }
    addNodeFromData(dataType: string, data: any): Node {
        return parseSkydiveMessageWithOneNode(this, data);
    }
    removeNodeFromData(dataType: string, data: any) {
        const nodeID = getNodeIDFromSkydiveMessageWithOneNode(data);
        this.nodeManager.removeNodeByID(nodeID);
        this.groupManager.removeNodeByID(nodeID);
        this.edgeManager.removeEdgeByNodeID(nodeID);
    }
    updateNodeFromData(dataType: string, data: any): any {
        const nodeID = getNodeIDFromSkydiveMessageWithOneNode(data);
        const node = this.nodeManager.getNodeById(nodeID);
        const clonedOldNode = node.clone();
        parseSkydiveMessageWithOneNodeAndUpdateNode(node, data);
        return { oldNode: clonedOldNode, newNode: node };
    }
    removeAllNodesWhichBelongsToHostFromData(dataType: string, data: any): void {
        const nodeHost = getHostFromSkydiveMessageWithOneNode(data);
        this.nodeManager.removeNodeByHost(nodeHost);
        this.groupManager.removeByHost(nodeHost);
    }
    removeAllEdgesWhichBelongsToHostFromData(dataType: string, data: any): void {
        const nodeHost = getHostFromSkydiveMessageWithOneNode(data);
        this.edgeManager.removeByHost(nodeHost);
    }
    updateFromData(dataType: string, data: any): void {
        parseData(this, dataType, data);
    }
    removeOldData(): void {
        this.nodeManager.removeOldData();
        this.edgeManager.removeOldData();
        this.groupManager.removeOldData();
    }

    updateEdgeFromData(sourceType: string, data: any) {
        const edgeID = getEdgeIDFromSkydiveMessageWithOneEdge(data);
        const edge = this.edgeManager.getEdgeById(edgeID);
        const clonedOldEdge = edge.clone();
        parseSkydiveMessageWithOneEdgeAndUpdateEdge(edge, data);
        return { oldEdge: clonedOldEdge, newEdge: edge };
    }

    addEdgeFromData(sourceType: string, data: any): Edge {
        return parseNewSkydiveEdgeAndUpdateDataManager(this, data);
    }

    removeEdgeFromData(sourceType: string, data: any): Edge {
        const edgeID = getEdgeIDFromSkydiveMessageWithOneEdge(data);
        const e = this.edgeManager.getEdgeById(edgeID);
        this.edgeManager.removeEdgeByID(edgeID);
        this.nodeManager.removeEdgeByID(edgeID);
        return e;
    }
}
