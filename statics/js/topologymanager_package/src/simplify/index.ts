import BaseSimplificationStrategy from './base';

import StructureSimplificationStrategy from "./structure";
import MergeNodesWithTheSameNameStrategy from "./merge_nodes_with_the_same_name"
import FlattenSamplesOfTopologyStrategy from "./flatten_parts_of_topology";

import * as TR from "./transformation";

export const TransformationRegistry = TR.default;

export function makeSimplificationStrategy(strategyType: any): BaseSimplificationStrategy {
    const typeToStrategy: any = {
        structure: StructureSimplificationStrategy,
        merge_similar_nodes: MergeNodesWithTheSameNameStrategy,
        flat_samples_of_topology: FlattenSamplesOfTopologyStrategy
    }
    return new typeToStrategy[strategyType]();
}
