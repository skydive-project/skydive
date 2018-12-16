import GraphRegistry, { Graph } from './graph_registry';

export default class HierarchicalGraph {
    width: number;
    height: number;
    nodeWidth: number = 150;
    children: GraphRegistry = new GraphRegistry();

    get depth() {
        return this.children.depth;
    }

    get length() {
        return this.children.length;
    }

    balance() {
        this.fixDepthForLastVertex();
        this.children.sortByLength();
        this.putVertexWithSpecificTypeToTheEndOfGraph();
        this.putVertexesRightBelowParents();
        this.stretchGraphVertically();
        this.calculateCoordinatesForVertices();
    }

    fixDepthForLastVertex() {
        const maxDepth = this.depth;
        this.children.graphs.forEach(g => {
            if (g.depth == maxDepth) {
                return;
            }
            g.fixDepthForLastVertex(maxDepth);
        });
    }

    putVertexWithSpecificTypeToTheEndOfGraph() {
        const maxDepth = this.depth + 1;
        this.children.graphs.forEach(g => {
            g.putVertexWithSpecificTypeToTheEndOfGraph(maxDepth, "dpdk");
            g.putVertexWithSpecificTypeToTheEndOfGraph(maxDepth, "device");
        });
    }

    putVertexesRightBelowParents() {
        this.children.graphs.forEach(g => {
            g.putVertexesRightBelowParents();
        });
    }

    stretchGraphVertically() {
        const tLength = this.width / this.nodeWidth;
        if (this.length > tLength) {
            return;
        }
        this.children.graphs.forEach(g => {
            g.balanceForLength(tLength);
        });
    }

    calculateCoordinatesForVertices() {
        const xMultiplier = this.nodeWidth;
        const yMultiplier = this.nodeWidth / 1.5;
        console.log('number of graphs', this.children.graphs.length);
        this.children.graphs.forEach((g: Graph, i: number) => {
            g.calculateCoordinatesForVertices(xMultiplier, yMultiplier, i === 0 ? this.nodeWidth / 2 : this.children.graphs[i - 1].getLastXCoord() + this.nodeWidth, 0);
        });
    }

    isXCoordBusy(x: number, minHorizontal: number, maxHorizontal: number, ignoreVIds: Array<string>) {
        return this.children.graphs.some((g: any) => {
            return g.isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds);
        });
    }

    suggestBetterFreeXCoord(x: number, minHorizontal: number, maxHorizontal: number, ignoreVIds: Array<string>) {
        const graph = this.children.graphs.find((g: any) => {
            return g.isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds);
        });
        return graph.suggestBetterFreeXCoord(x, minHorizontal, maxHorizontal);
    }

}
