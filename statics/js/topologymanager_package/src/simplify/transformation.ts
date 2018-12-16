import * as deepAssign from 'deep-assign';

const transformationHandlersPerType: any = {};
const mergeNodeStrategies: any = {};

export default class TransformationRegistry {

    static registerTransformationWay(dataFormat: any, handler: (data: any, deadNodes: Array<String>) => {}) {
        transformationHandlersPerType[dataFormat] = handler;
    }

    static getTransformationWay(dataFormat: any) {
        return transformationHandlersPerType[dataFormat];
    }

    static getMergeNodeStrategy(dataFormat: any) {
        return mergeNodeStrategies[dataFormat];
    }

    static registerMergeNodeStrategy(dataFormat: any, handler: (data: any, similarNodes: any) => {}) {
        mergeNodeStrategies[dataFormat] = handler;
    }

}

TransformationRegistry.registerTransformationWay(
    "skydive",
    (data: any, deadNodes: Array<String>) => {
        data.Obj.Nodes = data.Obj.Nodes.filter((node: any) => {
            return deadNodes.indexOf(node.ID) === -1;
        });
        data.Obj.Edges = data.Obj.Edges.filter((edge: any) => {
            return !(deadNodes.indexOf(edge.Parent) !== -1 || deadNodes.indexOf(edge.Child) !== -1);
        });
        return data;
    }
);


TransformationRegistry.registerMergeNodeStrategy(
    "skydive",
    (data: any, similarNodes: any) => {
        data.Obj.Edges = data.Obj.Edges.filter((edge: any) => {
            if (!similarNodes[edge.Parent] && !similarNodes[edge.Child]) {
                return true;
            }
            if (similarNodes[edge.Parent] && similarNodes[edge.Child] && similarNodes[edge.Parent] === similarNodes[edge.Child]) {
                return false;
            }
            if (similarNodes[edge.Parent]) {
                edge.Parent = similarNodes[edge.Parent];
            }
            if (similarNodes[edge.Child]) {
                edge.Child = similarNodes[edge.Child];
            }
            if (edge.Parent === edge.Child) {
                return false;
            }
            return true;
        });
        data.Obj.Nodes = data.Obj.Nodes.filter((node: any) => {
            return !!!similarNodes[node.ID];
        });
        return data;
    }
);
