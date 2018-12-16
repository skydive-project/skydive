export { default as HierarchicalGraph } from './hierarchical_graph';
import { Graph } from './graph_registry';
export { Graph } from './graph_registry';

export function makeGraph() {
    return new Graph();
}
