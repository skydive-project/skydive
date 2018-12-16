import LayoutConfig from '../../config';
import { HierarchicalGraph, makeGraph, Graph } from '../../../algorithms/index';


export default class FlattenNodesForHierarchyAspect {
    graph: HierarchicalGraph = new HierarchicalGraph();
    hostID: string;

    constructor(config: LayoutConfig) {
        this.graph = new HierarchicalGraph();
    }

    recurseFlattening(graph: Graph, vertical: number, node: any, parent: any, depth: number) {
        node.childrenIds && node.childrenIds.delete && node.childrenIds.delete(this.hostID);
        node.depth = depth;
        node.horizontal = depth;
        node.fixed = true;
        node.width = 33;
        graph.addVertex(node);
        if (node.children) {
            node.children.forEach((children: any) => {
                this.recurseFlattening(graph, vertical, children, node, depth + 1);
            });
        }
    }

    flattenForWidthAndHeight(hostID: any, data: any, width: number, height: number): Array<any> {

        this.hostID = hostID;
        this.graph.width = width;
        this.graph.height = height;
        data.children.forEach((nodeVm: any, vertical: number) => {
            const graph = makeGraph();
            this.graph.children.addGraph(graph);
            this.recurseFlattening(graph, vertical, nodeVm, null, 0);
        });
        this.graph.children.mergeGraphsWithCommonVertexes();
        console.log('number of graphs', this.graph.children.graphs.length);
        this.graph.balance();
        const nodes = this.graph.children.getVertexes();
        console.log('NODES AFTER GRAPH BALANCING', nodes);
        nodes.sort((nodeA: any, nodeB: any) => {
            if (nodeA.depth != nodeB.depth) {
                return nodeA.depth - nodeB.depth;
            }
            return nodeA.x - nodeB.x;
        });
        return nodes;
    }

}
