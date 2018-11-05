import { NodeRegistry } from './node/index';
import { EdgeRegistry } from './edge/index';
import { GroupRegistry } from './group/index';
import LayoutContext from './ui/layout_context';
import { Node } from './node/index';
import parseData, { parseSkydiveMessageWithOneNodeAndUpdateNode, parseSkydiveMessageWithOneNode, getNodeIDFromSkydiveMessageWithOneNode, getHostFromSkydiveMessageWithOneNode } from './parsers/index';
export default class DataManager {
    nodeManager: NodeRegistry = new NodeRegistry();
    edgeManager: EdgeRegistry = new EdgeRegistry();
    groupManager: GroupRegistry = new GroupRegistry();
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
    }
    addNodeFromData(dataType: string, data: any) {
        parseSkydiveMessageWithOneNode(this, data);
    }
    removeNodeFromData(dataType: string, data: any) {
        const nodeID = getNodeIDFromSkydiveMessageWithOneNode(data);
        this.nodeManager.removeNodeByID(nodeID);
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
    }
    updateFromData(dataType: string, data: any): void {
        parseData(this, dataType, data);
    }
    removeOldData(): void {
        this.nodeManager.removeOldData();
        this.edgeManager.removeOldData();
        this.groupManager.removeOldData();
    }
}
