/******/ (function(modules) { // webpackBootstrap
/******/ 	// The module cache
/******/ 	var installedModules = {};
/******/
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/
/******/ 		// Check if module is in cache
/******/ 		if(installedModules[moduleId]) {
/******/ 			return installedModules[moduleId].exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = installedModules[moduleId] = {
/******/ 			i: moduleId,
/******/ 			l: false,
/******/ 			exports: {}
/******/ 		};
/******/
/******/ 		// Execute the module function
/******/ 		modules[moduleId].call(module.exports, module, module.exports, __webpack_require__);
/******/
/******/ 		// Flag the module as loaded
/******/ 		module.l = true;
/******/
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/
/******/
/******/ 	// expose the modules object (__webpack_modules__)
/******/ 	__webpack_require__.m = modules;
/******/
/******/ 	// expose the module cache
/******/ 	__webpack_require__.c = installedModules;
/******/
/******/ 	// define getter function for harmony exports
/******/ 	__webpack_require__.d = function(exports, name, getter) {
/******/ 		if(!__webpack_require__.o(exports, name)) {
/******/ 			Object.defineProperty(exports, name, {
/******/ 				configurable: false,
/******/ 				enumerable: true,
/******/ 				get: getter
/******/ 			});
/******/ 		}
/******/ 	};
/******/
/******/ 	// getDefaultExport function for compatibility with non-harmony modules
/******/ 	__webpack_require__.n = function(module) {
/******/ 		var getter = module && module.__esModule ?
/******/ 			function getDefault() { return module['default']; } :
/******/ 			function getModuleExports() { return module; };
/******/ 		__webpack_require__.d(getter, 'a', getter);
/******/ 		return getter;
/******/ 	};
/******/
/******/ 	// Object.prototype.hasOwnProperty.call
/******/ 	__webpack_require__.o = function(object, property) { return Object.prototype.hasOwnProperty.call(object, property); };
/******/
/******/ 	// __webpack_public_path__
/******/ 	__webpack_require__.p = "";
/******/
/******/ 	// Load entry module and return exports
/******/ 	return __webpack_require__(__webpack_require__.s = 7);
/******/ })
/************************************************************************/
/******/ ([
/* 0 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
const transformationHandlersPerType = {};
const mergeNodeStrategies = {};
class TransformationRegistry {
    static registerTransformationWay(dataFormat, handler) {
        transformationHandlersPerType[dataFormat] = handler;
    }
    static getTransformationWay(dataFormat) {
        return transformationHandlersPerType[dataFormat];
    }
    static getMergeNodeStrategy(dataFormat) {
        return mergeNodeStrategies[dataFormat];
    }
    static registerMergeNodeStrategy(dataFormat, handler) {
        mergeNodeStrategies[dataFormat] = handler;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = TransformationRegistry;

TransformationRegistry.registerTransformationWay("skydive", (data, deadNodes) => {
    data.Obj.Nodes = data.Obj.Nodes.filter((node) => {
        return deadNodes.indexOf(node.ID) === -1;
    });
    data.Obj.Edges = data.Obj.Edges.filter((edge) => {
        return !(deadNodes.indexOf(edge.Parent) !== -1 || deadNodes.indexOf(edge.Child) !== -1);
    });
    return data;
});
TransformationRegistry.registerMergeNodeStrategy("skydive", (data, similarNodes) => {
    data.Obj.Edges = data.Obj.Edges.filter((edge) => {
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
    data.Obj.Nodes = data.Obj.Nodes.filter((node) => {
        return !!!similarNodes[node.ID];
    });
    return data;
});


/***/ }),
/* 1 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["b"] = transformSkydiveDataForSimplification;
/* harmony export (immutable) */ __webpack_exports__["a"] = prepareDataForHierarchyLayout;
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_deep_assign__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_deep_assign___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_deep_assign__);

/**
Transforms data from skydive format into format needed for
 **simplification
We need to have a graph with a following structure. Each root should
 **be similar to {
  "Host": HostName,
  "Name": NodeName,
  "Type": NodeType,
  "children": set of children nodes,
  "parent": set of parent nodes,
  "layers": set of layered nodes
}
**/
function transformSkydiveDataForSimplification(data) {
    let simplifyData = {};
    data.Obj.Nodes.forEach((item) => {
        simplifyData[item.ID] = {
            Host: item.Host,
            Name: item.Metadata.Name,
            Type: item.Metadata.Type,
            Driver: item.Metadata.Driver,
            children: new Set([]),
            parent: new Set([]),
            layers: new Set([])
        };
    });
    data.Obj.Edges.forEach((item) => {
        // this is some kind of layer between two nodes, so lets make
        // this node to have both nodes as parent and children
        if (item.Metadata.RelationType !== "ownership") {
            simplifyData[item.Child].layers.add(item.Parent);
            simplifyData[item.Parent].layers.add(item.Child);
        }
        else {
            simplifyData[item.Parent].children.add(item.Child);
            simplifyData[item.Child].parent.add(item.Parent);
        }
    });
    return simplifyData;
}
function recursivelyBuildHierarchy(parentNode, nodesInfo, data) {
    if (parentNode.processed) {
        return;
    }
    parentNode.processed = true;
    let children = new Set([...parentNode.childrenIds]);
    // remove duplicated nodes from skydive hierarchy
    children.forEach((nodeId) => {
        const nodeInfo = nodesInfo[nodeId];
        if (parentNode.Type !== "libvirt" && nodeInfo.Name === parentNode.Name) {
            children.delete(nodeId);
            parentNode.childrenIds.delete(nodeId);
            nodeInfo.childrenIds.forEach((nodeId1) => {
                children.add(nodeId1);
                parentNode.childrenIds.add(nodeId1);
            });
            nodeInfo.layerIds.forEach((nodeId1) => {
                children.add(nodeId1);
                parentNode.childrenIds.add(nodeId1);
            });
        }
    });
    children.forEach((nodeID) => {
        const nodeInfo = nodesInfo[nodeID];
        if (!nodeInfo) {
            return;
        }
        if (nodeInfo) {
            if (nodeInfo.Type !== "host") {
                parentNode.children.push(__WEBPACK_IMPORTED_MODULE_0_deep_assign__(nodeInfo, data[nodeID]));
            }
        }
        recursivelyBuildHierarchy(nodesInfo[nodeID], nodesInfo, data);
    });
}
function prepareDataForHierarchyLayout(initialData) {
    const data = transformSkydiveDataForSimplification(initialData);
    const node = {
        Type: "tophierarchy",
        Name: "Hierarchy top",
        ID: "Tophierarchy",
        childrenIds: new Set([]),
        children: new Array(),
        layerIds: new Set([]),
        processed: false
    };
    const nodesInfo = {};
    nodesInfo[node.ID] = node;
    let nodes = [];
    const nodeTypesToCheck = ["libvirt"];
    const skipHosts = new Set([]);
    // find initial list of nodes, actually, its vm nodes
    Object.keys(data).forEach((nodeId) => {
        if (nodeTypesToCheck.indexOf(data[nodeId].Type) === -1) {
            return;
        }
        nodes.push(nodeId);
    });
    let i = 0;
    let nodesCount = Object.keys(data).length;
    const processedNodes = new Set([]);
    while (true) {
        const nodeId = nodes[i];
        const nodeData = data[nodeId];
        if (!nodeData) {
            break;
        }
        let goThroughNodes = [];
        try {
            if (!nodesInfo[nodeId]) {
                nodesInfo[nodeId] = {
                    Type: nodeData.Type,
                    childrenIds: new Set([]),
                    ID: nodeId,
                    children: [],
                    layerIds: new Set([]),
                    processed: false,
                    Name: nodeData.Name
                };
            }
            if (nodeData.Type === "libvirt") {
                node.childrenIds.add(nodeId);
            }
            if (nodeData.Type === "host") {
                continue;
            }
            // sometimes nodes are not connected directly but
            // connected through layer2 relations, its important
            // for us as well, for instance, here is a typical
            // connection from skydive data structure
            // {
            // "ID":"08883be7-f8be-583d-4bf8-fcd3e611a05f",
            // "Metadata":{
            // "RelationType":"layer2"
            // },
            // "Parent":"ec13216d-af7e-42df-5efe-a478733545de",
            // "Child":"71ace516-0ade-432f-4531-4b5e2a828ced",
            // "Host":"DELL2",
            // "CreatedAt":1524827146311,
            // "UpdatedAt":1524827146311
            // },
            // it needs to be treated as well to make sure we
            // don't lose any information from topology
            if (nodeData.layers.size) {
                const nodesToBeAdded = nodeData.layers;
                nodesToBeAdded.forEach((nodeId1) => {
                    nodesInfo[nodeId].childrenIds.add(nodeId1);
                    nodeData.children.add(nodeId1);
                    data[nodeId1].layers.delete(nodeId);
                    data[nodeId1].children.delete(nodeId);
                    if (nodesInfo[nodeId1]) {
                        nodesInfo[nodeId1].layerIds.delete(nodeId);
                        nodesInfo[nodeId1].childrenIds.delete(nodeId);
                    }
                });
                goThroughNodes = goThroughNodes.concat([...nodesToBeAdded]);
            }
            // handle list of children of specific node
            if (nodeData.children.size) {
                const childrenToBeAdded = nodeData.children;
                if (childrenToBeAdded.size) {
                    goThroughNodes = goThroughNodes.concat([...childrenToBeAdded]);
                }
            }
            // handle list of parent of specific node
            if (nodeData.parent.size) {
                const parentsToBeAdded = nodeData.parent;
                if (parentsToBeAdded.size) {
                    parentsToBeAdded.forEach((nodeId1) => {
                        nodesInfo[nodeId].childrenIds.add(nodeId1);
                    });
                    goThroughNodes = goThroughNodes.concat([...parentsToBeAdded]);
                }
            }
        }
        finally {
            // break if total list of handled nodes equal total
            // size of topology
            if (processedNodes.size === nodesCount) {
                break;
            }
            // extend list of nodes to be visited
            if (goThroughNodes.length) {
                const uniqueNodes = new Set(goThroughNodes);
                let uniqueNodes1 = [...uniqueNodes];
                nodes = nodes.concat(uniqueNodes1);
            }
            processedNodes.add(nodeId);
            ++i;
        }
    }
    const IDToData = {};
    initialData.Obj.Nodes.forEach((node) => {
        IDToData[node.ID] = node;
    });
    recursivelyBuildHierarchy(node, nodesInfo, IDToData);
    // @todo, this part should be fixed, it's a workaround to fix a
    // problem with endless loop and circular structure in javascript
    nodesInfo["Tophierarchy"].children.forEach((node) => {
        fixProblemWithRecursionInData(node, node.children, [node.ID]);
    });
    return nodesInfo["Tophierarchy"];
}
function fixProblemWithRecursionInData(parentNode, children, nodesInHierarchy) {
    children.forEach((child) => {
        child.children = child.children.filter((child1) => {
            return nodesInHierarchy.indexOf(child1.ID) === -1;
        });
        child.childrenIds = new Set([...child.childrenIds].filter((child1) => {
            return nodesInHierarchy.indexOf(child1) === -1;
        }));
        nodesInHierarchy.push(child.ID);
        fixProblemWithRecursionInData(child, child.children, nodesInHierarchy);
    });
}


/***/ }),
/* 2 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__iterator_helpers__ = __webpack_require__(23);

class Graph {
    constructor() {
        this.vertices = {};
    }
    get depth() {
        return Math.max(...Object.keys(this.vertices).map((vId) => this.vertices[vId].getHorizontal() || 0));
    }
    get length() {
        const numberOfVerticesPerHorizontals = {};
        Object.keys(this.vertices).forEach((vId) => {
            if (!numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()]) {
                numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()] = 0;
            }
            ++numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()];
        });
        return Math.max(...Object.keys(numberOfVerticesPerHorizontals).map((h) => numberOfVerticesPerHorizontals[h]));
    }
    addVertex(vertexdata) {
        this.vertices[vertexdata.ID] = new Vertex(vertexdata);
    }
    getVertexes() {
        return Object.keys(this.vertices).map(vId => {
            return this.vertices[vId].getData();
        });
    }
    fixDepthForLastVertex(depth) {
        const graphDepth = this.depth;
        Object.keys(this.vertices).forEach(vId => {
            if (this.vertices[vId].getHorizontal() < graphDepth) {
                return;
            }
            this.vertices[vId].setHorizontal(depth);
        });
    }
    putVertexWithSpecificTypeToTheEndOfGraph(depth, t) {
        Object.keys(this.vertices).forEach(vId => {
            if (this.vertices[vId].getType() !== t) {
                return;
            }
            if (this.vertices[vId].getChildren().length !== 0) {
                return;
            }
            this.vertices[vId].setHorizontal(depth);
        });
    }
    putVertexesRightBelowParents() {
        const currentVerticalPerHorizontal = {};
        while (true) {
            const foundAnyVertexWithNotDefinedVertical = Object.keys(this.vertices).some(vId => {
                return this.vertices[vId].getVertical() === undefined;
            });
            if (!foundAnyVertexWithNotDefinedVertical) {
                break;
            }
            Object(__WEBPACK_IMPORTED_MODULE_0__iterator_helpers__["a" /* range */])(this.depth, 0).forEach((h) => {
                const lengthForHorizontal = this.lengthForHorizontal(h);
                const lengthPerVertex = this.length / lengthForHorizontal;
                Object.keys(this.vertices).forEach(vId => {
                    if (h !== this.vertices[vId].getHorizontal()) {
                        return;
                    }
                    if (currentVerticalPerHorizontal[h] === undefined) {
                        currentVerticalPerHorizontal[h] = 0;
                    }
                    if (this.vertices[vId].getVertical() === undefined) {
                        this.setVerticalForVertex(this.vertices[vId], currentVerticalPerHorizontal[h], lengthPerVertex);
                        currentVerticalPerHorizontal[h] = this.vertices[vId].getVertical() + lengthPerVertex;
                    }
                    const firstVertexId = this.vertices[vId].getChildren().length ? this.vertices[vId].getChildren()[0].ID : null;
                    this.vertices[vId].getChildren().forEach((c) => {
                        c = this.vertices[c.ID];
                        if (c.getVertical() !== undefined) {
                            return;
                        }
                        const lengthForHorizontal1 = this.lengthForHorizontal(c.getHorizontal());
                        const lengthPerVertex1 = this.length / lengthForHorizontal1;
                        if (currentVerticalPerHorizontal[h + 1] === undefined) {
                            currentVerticalPerHorizontal[h + 1] = 0;
                        }
                        if (firstVertexId === c.getId()) {
                            let isTakenHorizontalInfo;
                            let vert = this.vertices[vId].getVertical();
                            while (true) {
                                isTakenHorizontalInfo = this.isThisVerticalTakenOnHorizontal(c.getHorizontal(), c.getId(), vert, 1);
                                if (isTakenHorizontalInfo.taken) {
                                    console.log('taken verticals for', isTakenHorizontalInfo, vert, this.vertices[vId].getVertical());
                                    vert = vert + 2;
                                    continue;
                                }
                                if (!isTakenHorizontalInfo.taken) {
                                    if (this.vertices[vId].getVertical() !== vert) {
                                        this.vertices[vId].setVertical(vert);
                                    }
                                    break;
                                }
                                if (isTakenHorizontalInfo.taken) {
                                    isTakenHorizontalInfo = this.isThisVerticalTakenOnHorizontal(c.getHorizontal(), c.getId(), this.vertices[vId].getVertical(), lengthPerVertex1);
                                }
                            }
                            console.log('taken verticals', isTakenHorizontalInfo, vert, this.vertices[vId].getVertical());
                            this.vertices[vId].fixVertical();
                            let verticalToBeAssigned = Math.min(this.vertices[vId].getVertical(), currentVerticalPerHorizontal[h + 1] + lengthPerVertex1);
                            this.clearVerticalsForHorizontal(c.getHorizontal(), c.getId(), verticalToBeAssigned);
                            this.setVerticalForFirstChildVertex(this.vertices[vId], c, verticalToBeAssigned, lengthPerVertex1);
                            c.fixVertical();
                            this.clearVerticalsForHorizontal(this.vertices[vId].getHorizontal(), vId, verticalToBeAssigned);
                            if (c.getVertical() === undefined) {
                                return;
                            }
                            currentVerticalPerHorizontal[h] = c.getVertical() + lengthPerVertex;
                            if (c.getVertical() === undefined) {
                                return;
                            }
                        }
                        else {
                            this.setVerticalForVertex(c, currentVerticalPerHorizontal[h + 1], lengthPerVertex1);
                            if (c.getVertical() === undefined) {
                                return;
                            }
                        }
                        currentVerticalPerHorizontal[h + 1] = c.getVertical() + lengthPerVertex1;
                    });
                });
            });
        }
    }
    balanceForLength(length) {
        const k = this.length / length;
        Object.keys(this.vertices).forEach(vId => {
            // this.vertices[vId].setVertical(k * this.vertices[vId].getVertical());
        });
    }
    lengthForHorizontal(h) {
        return Object.keys(this.vertices).reduce((accum, vId) => {
            const v = this.vertices[vId];
            if (v.getHorizontal() !== h) {
                return accum;
            }
            ++accum;
            return accum;
        }, 0);
    }
    calculateCoordinatesForVertices(xMultiplier, yMultiplier, xOffset, yOffset) {
        Object.keys(this.vertices).forEach((vId) => {
            const v = this.vertices[vId];
            v.setXCoord(xMultiplier * v.getVertical() + xOffset);
            v.setYCoord(yMultiplier * v.getHorizontal() + yOffset);
        });
    }
    verticesIds() {
        return new Set(Object.keys(this.vertices));
    }
    intersects(graph) {
        const verticesIds = this.verticesIds();
        const anotherVerticesIds = graph.verticesIds();
        const intersection = new Set([...verticesIds].filter(x => anotherVerticesIds.has(x)));
        return !!intersection.size;
    }
    merge(graph) {
        this.vertices = Object.keys(graph.vertices).reduce((accum, vId) => {
            accum[vId] = graph.vertices[vId];
            return accum;
        }, this.vertices);
    }
    clearVerticals(vertices) {
        if (!vertices) {
            return;
        }
        vertices.forEach((v) => {
            this.vertices[v.ID].clearVertical();
            this.clearVerticals(this.vertices[v.ID].getChildren());
        });
    }
    clearVerticalsForHorizontal(h, vId, vertical) {
        Object.keys(this.vertices).forEach(vId1 => {
            if (vId1 === vId) {
                return;
            }
            if (h !== this.vertices[vId1].getHorizontal()) {
                return;
            }
            if (this.vertices[vId1].getVertical() < vertical) {
                return;
            }
            this.vertices[vId1].clearVertical();
            this.clearVerticals(this.vertices[vId1].getChildren());
        });
    }
    isThisVerticalTakenOnHorizontal(horizontal, ignoreVId, verticalToLookFor, delta) {
        const minVertical = verticalToLookFor - delta;
        const maxVertical = verticalToLookFor + delta;
        const verticals = Object.keys(this.vertices).reduce((accum, vId1) => {
            if (this.vertices[vId1].getHorizontal() !== horizontal) {
                return accum;
            }
            if (ignoreVId === vId1) {
                return accum;
            }
            if (this.vertices[vId1].getVertical() === undefined) {
                return accum;
            }
            const vertexVertical = this.vertices[vId1].getVertical();
            if (vertexVertical > minVertical && vertexVertical < maxVertical) {
                accum.push(vertexVertical);
            }
            return accum;
        }, []);
        if (verticals.length) {
            return { taken: true, vertical: Math.max(...verticals) };
        }
        return { taken: false };
    }
    isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds) {
        return Object.keys(this.vertices).some(vId => {
            if (this.vertices[vId].getHorizontal() >= maxHorizontal) {
                return false;
            }
            if (this.vertices[vId].getHorizontal() <= minHorizontal) {
                return false;
            }
            if (ignoreVIds.indexOf(vId) !== -1) {
                return false;
            }
            return this.vertices[vId].getXCoord() === x;
        });
    }
    suggestBetterFreeXCoord(x, minHorizontal, maxHorizontal) {
        return x + 10;
    }
    isVerticalTakenByAnyVertex(horizontal, verticalToCheck, delta) {
        const vertexes = [];
        Object.keys(this.vertices).forEach(vId => {
            if (this.vertices[vId].getHorizontal() !== horizontal) {
                return;
            }
            const minVertical = verticalToCheck - delta;
            const maxVertical = verticalToCheck + delta;
            const vertexVertical = this.vertices[vId].getVertical();
            if (vertexVertical === undefined) {
                return;
            }
            if (vertexVertical > minVertical && vertexVertical < maxVertical) {
                vertexes.push(this.vertices[vId]);
            }
        });
        if (!vertexes.length) {
            return false;
        }
        const vertical = Math.max(...vertexes.map(v => v.getVertical()));
        return vertexes.find(v => v.getVertical() === vertical);
    }
    suggestVerticalToPlaceVertexOnHorizontal(vertex, verticalToCheck, delta) {
        const horizontal = vertex.getHorizontal();
        let verticalToSuggest = null;
        const vertexes = [];
        Object.keys(this.vertices).forEach(vId => {
            if (this.vertices[vId].getHorizontal() !== horizontal) {
                return;
            }
            const minVertical = verticalToCheck - delta;
            const maxVertical = verticalToCheck + delta;
            const vertexVertical = this.vertices[vId].getVertical();
            if (vertexVertical !== undefined && vertexVertical > minVertical && vertexVertical < maxVertical) {
                vertexes.push(this.vertices[vId]);
            }
        });
        if (!vertexes.length) {
            return verticalToCheck;
        }
        const vertical = Math.max(...vertexes.map(v => v.getVertical()));
        return vertical + delta;
    }
    setVerticalForVertex(vertex, vertical, delta) {
        const vertexPlacedOnThisVertical = this.isVerticalTakenByAnyVertex(vertex.getHorizontal(), vertical, delta);
        if (vertexPlacedOnThisVertical) {
            vertex.setVertical(this.suggestVerticalToPlaceVertexOnHorizontal(vertex, vertical, delta));
        }
        else {
            vertex.setVertical(vertical);
        }
        console.log('set vertical for node', vertex.getHorizontal(), vertex.getName(), vertical, vertex.getVertical(), delta);
    }
    setVerticalForFirstChildVertex(parentVertex, vertex, vertical, delta) {
        if (parentVertex.isFixedVertical()) {
            const vertexPlacedOnParentVertical = this.isVerticalTakenByAnyVertex(vertex.getHorizontal(), parentVertex.getVertical(), delta);
            vertex.setVertical(parentVertex.getVertical());
            return;
            // if (vertexPlacedOnParentVertical) {
            //     const vert = this.suggestVerticalToPlaceVertexOnHorizontal(parentVertex, parentVertex.getVertical(), delta);
            //     console.log('vertex we trying to place', vertex.getName(), vertexPlacedOnParentVertical ? vertexPlacedOnParentVertical.getName() : null, vert, parentVertex.getVertical(), delta);
            //     vertex.setVertical(vert);
            //     parentVertex.setVertical(vert);
            // } else {
            //     console.log('vertex we trying to place', vertex.getName(), vertexPlacedOnParentVertical ? vertexPlacedOnParentVertical.getName() : null, parentVertex.getVertical(), delta);
            //     vertex.setVertical(parentVertex.getVertical());
            // }
            // return;
        }
        const vertexPlacedOnParentVertical = this.isVerticalTakenByAnyVertex(vertex.getHorizontal(), parentVertex.getVertical(), delta);
        const vertexPlacedOnThisVertical = this.isVerticalTakenByAnyVertex(vertex.getHorizontal(), vertical, delta);
        const theoreticalVerticals = [];
        let vert = null;
        if (vertexPlacedOnParentVertical) {
            vert = this.suggestVerticalToPlaceVertexOnHorizontal(parentVertex, parentVertex.getVertical(), delta);
            if (vert) {
                theoreticalVerticals.push(vert);
            }
        }
        if (vertexPlacedOnThisVertical) {
            vert = this.suggestVerticalToPlaceVertexOnHorizontal(vertex, vertical, delta);
            if (vert) {
                theoreticalVerticals.push(vert);
            }
        }
        else {
            theoreticalVerticals.push(vertical);
        }
        const vertical1 = Math.min(...theoreticalVerticals);
        vertex.setVertical(vertical1);
        console.log('set vertical for node', vertex.getHorizontal(), vertex.getName(), vertical, vertex.getVertical(), delta);
        parentVertex.setVertical(vertex.getVertical());
    }
    getLastXCoord() {
        return Math.max(...Object.keys(this.vertices).map(vId => this.vertices[vId].getXCoord()));
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = Graph;

class GraphRegistry {
    constructor() {
        this.graphs = [];
    }
    addGraph(graph) {
        this.graphs.push(graph);
    }
    get depth() {
        return Math.max(...this.graphs.map(g => g.depth));
    }
    get length() {
        return this.graphs.reduce((accum, g) => {
            accum += g.length;
            return accum;
        }, 0);
    }
    sortByLength() {
        this.graphs.sort((a, b) => {
            return b.length - a.length;
        });
    }
    getVertexes() {
        return this.graphs.reduce((accum, g) => {
            accum = accum.concat(g.getVertexes());
            return accum;
        }, []);
    }
    mergeGraphsWithCommonVertexes() {
        const mergedGraphsWithAnother = [];
        const addedGraphs = [];
        const mergedGraphs = [];
        this.graphs.forEach((g, i) => {
            if (mergedGraphsWithAnother.indexOf(i) !== -1) {
                return;
            }
            this.graphs.forEach((g1, j) => {
                if (mergedGraphsWithAnother.indexOf(j) !== -1) {
                    return;
                }
                if (i === j) {
                    if (mergedGraphsWithAnother.indexOf(i) !== -1) {
                        return;
                    }
                    if (addedGraphs.indexOf(i) === -1) {
                        mergedGraphs.push(g);
                        addedGraphs.push(i);
                    }
                    return;
                }
                if (g.intersects(g1)) {
                    g.merge(g1);
                    mergedGraphsWithAnother.push(j);
                    mergedGraphsWithAnother.push(i);
                }
                if (addedGraphs.indexOf(i) === -1) {
                    mergedGraphs.push(g);
                    addedGraphs.push(i);
                }
            });
        });
        this.graphs = mergedGraphs;
    }
}
/* harmony export (immutable) */ __webpack_exports__["b"] = GraphRegistry;

class Vertex {
    constructor(data) {
        this.data = data;
        if (!this.data.id) {
            this.data.id = this.data.ID;
        }
    }
    getData() {
        return this.data;
    }
    getName() {
        return this.data.Name;
    }
    getHorizontal() {
        return this.data.depth;
    }
    setHorizontal(depth) {
        this.data.depth = depth;
        this.data.horizontal = depth;
    }
    getVertical() {
        return this.data.vertical;
    }
    setVertical(vertical) {
        this.data.vertical = vertical;
    }
    clearVertical() {
        // delete this.data.fixedVertical;
        // delete this.data.vertical;
    }
    fixVertical() {
        this.data.fixedVertical = true;
    }
    isFixedVertical() {
        return !!this.data.fixedVertical;
    }
    getType() {
        return this.data.Type;
    }
    getId() {
        return this.data.ID;
    }
    setYCoord(y) {
        this.data.y = y;
    }
    setXCoord(x) {
        this.data.x = x;
    }
    getXCoord() {
        return this.data.x;
    }
    setWidth(width) {
        this.data.width = width;
    }
    getChildren() {
        return this.data.children;
    }
}


/***/ }),
/* 3 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class BaseSimplificationStrategy {
    simplifyData(dataFormat, data) {
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = BaseSimplificationStrategy;



/***/ }),
/* 4 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__node__ = __webpack_require__(13);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return __WEBPACK_IMPORTED_MODULE_0__node__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__layout__ = __webpack_require__(14);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return __WEBPACK_IMPORTED_MODULE_1__layout__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__group__ = __webpack_require__(15);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return __WEBPACK_IMPORTED_MODULE_2__group__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__link__ = __webpack_require__(16);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_3__link__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__bridge__ = __webpack_require__(17);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return __WEBPACK_IMPORTED_MODULE_4__bridge__["a"]; });







/***/ }),
/* 5 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";

var isObj = __webpack_require__(30);
var hasOwnProperty = Object.prototype.hasOwnProperty;
var propIsEnumerable = Object.prototype.propertyIsEnumerable;

function toObject(val) {
	if (val === null || val === undefined) {
		throw new TypeError('Sources cannot be null or undefined');
	}

	return Object(val);
}

function assignKey(to, from, key) {
	var val = from[key];

	if (val === undefined || val === null) {
		return;
	}

	if (hasOwnProperty.call(to, key)) {
		if (to[key] === undefined || to[key] === null) {
			throw new TypeError('Cannot convert undefined or null to object (' + key + ')');
		}
	}

	if (!hasOwnProperty.call(to, key) || !isObj(val)) {
		to[key] = val;
	} else {
		to[key] = assign(Object(to[key]), from[key]);
	}
}

function assign(to, from) {
	if (to === from) {
		return to;
	}

	from = Object(from);

	for (var key in from) {
		if (hasOwnProperty.call(from, key)) {
			assignKey(to, from, key);
		}
	}

	if (Object.getOwnPropertySymbols) {
		var symbols = Object.getOwnPropertySymbols(from);

		for (var i = 0; i < symbols.length; i++) {
			if (propIsEnumerable.call(from, symbols[i])) {
				assignKey(to, from, symbols[i]);
			}
		}
	}

	return to;
}

module.exports = function deepAssign(target) {
	target = toObject(target);

	for (var s = 1; s < arguments.length; s++) {
		assign(target, arguments[s]);
	}

	return target;
};


/***/ }),
/* 6 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * isobject <https://github.com/jonschlinkert/isobject>
 *
 * Copyright (c) 2014-2015, Jon Schlinkert.
 * Licensed under the MIT License.
 */



var isArray = __webpack_require__(35);

module.exports = function isObject(val) {
  return val != null && typeof val === 'object' && isArray(val) === false;
};


/***/ }),
/* 7 */
/***/ (function(module, exports, __webpack_require__) {

__webpack_require__(8);
__webpack_require__(9);
module.exports = __webpack_require__(40);


/***/ }),
/* 8 */
/***/ (function(module, exports) {

(function (global) {
  var babelHelpers = global.babelHelpers = {};
  babelHelpers.typeof = typeof Symbol === "function" && typeof Symbol.iterator === "symbol" ? function (obj) {
    return typeof obj;
  } : function (obj) {
    return obj && typeof Symbol === "function" && obj.constructor === Symbol && obj !== Symbol.prototype ? "symbol" : typeof obj;
  };

  babelHelpers.jsx = function () {
    var REACT_ELEMENT_TYPE = typeof Symbol === "function" && Symbol.for && Symbol.for("react.element") || 0xeac7;
    return function createRawReactElement(type, props, key, children) {
      var defaultProps = type && type.defaultProps;
      var childrenLength = arguments.length - 3;

      if (!props && childrenLength !== 0) {
        props = {};
      }

      if (props && defaultProps) {
        for (var propName in defaultProps) {
          if (props[propName] === void 0) {
            props[propName] = defaultProps[propName];
          }
        }
      } else if (!props) {
        props = defaultProps || {};
      }

      if (childrenLength === 1) {
        props.children = children;
      } else if (childrenLength > 1) {
        var childArray = Array(childrenLength);

        for (var i = 0; i < childrenLength; i++) {
          childArray[i] = arguments[i + 3];
        }

        props.children = childArray;
      }

      return {
        $$typeof: REACT_ELEMENT_TYPE,
        type: type,
        key: key === undefined ? null : '' + key,
        ref: null,
        props: props,
        _owner: null
      };
    };
  }();

  babelHelpers.asyncIterator = function (iterable) {
    if (typeof Symbol === "function") {
      if (Symbol.asyncIterator) {
        var method = iterable[Symbol.asyncIterator];
        if (method != null) return method.call(iterable);
      }

      if (Symbol.iterator) {
        return iterable[Symbol.iterator]();
      }
    }

    throw new TypeError("Object is not async iterable");
  };

  babelHelpers.asyncGenerator = function () {
    function AwaitValue(value) {
      this.value = value;
    }

    function AsyncGenerator(gen) {
      var front, back;

      function send(key, arg) {
        return new Promise(function (resolve, reject) {
          var request = {
            key: key,
            arg: arg,
            resolve: resolve,
            reject: reject,
            next: null
          };

          if (back) {
            back = back.next = request;
          } else {
            front = back = request;
            resume(key, arg);
          }
        });
      }

      function resume(key, arg) {
        try {
          var result = gen[key](arg);
          var value = result.value;

          if (value instanceof AwaitValue) {
            Promise.resolve(value.value).then(function (arg) {
              resume("next", arg);
            }, function (arg) {
              resume("throw", arg);
            });
          } else {
            settle(result.done ? "return" : "normal", result.value);
          }
        } catch (err) {
          settle("throw", err);
        }
      }

      function settle(type, value) {
        switch (type) {
          case "return":
            front.resolve({
              value: value,
              done: true
            });
            break;

          case "throw":
            front.reject(value);
            break;

          default:
            front.resolve({
              value: value,
              done: false
            });
            break;
        }

        front = front.next;

        if (front) {
          resume(front.key, front.arg);
        } else {
          back = null;
        }
      }

      this._invoke = send;

      if (typeof gen.return !== "function") {
        this.return = undefined;
      }
    }

    if (typeof Symbol === "function" && Symbol.asyncIterator) {
      AsyncGenerator.prototype[Symbol.asyncIterator] = function () {
        return this;
      };
    }

    AsyncGenerator.prototype.next = function (arg) {
      return this._invoke("next", arg);
    };

    AsyncGenerator.prototype.throw = function (arg) {
      return this._invoke("throw", arg);
    };

    AsyncGenerator.prototype.return = function (arg) {
      return this._invoke("return", arg);
    };

    return {
      wrap: function (fn) {
        return function () {
          return new AsyncGenerator(fn.apply(this, arguments));
        };
      },
      await: function (value) {
        return new AwaitValue(value);
      }
    };
  }();

  babelHelpers.asyncGeneratorDelegate = function (inner, awaitWrap) {
    var iter = {},
        waiting = false;

    function pump(key, value) {
      waiting = true;
      value = new Promise(function (resolve) {
        resolve(inner[key](value));
      });
      return {
        done: false,
        value: awaitWrap(value)
      };
    }

    ;

    if (typeof Symbol === "function" && Symbol.iterator) {
      iter[Symbol.iterator] = function () {
        return this;
      };
    }

    iter.next = function (value) {
      if (waiting) {
        waiting = false;
        return value;
      }

      return pump("next", value);
    };

    if (typeof inner.throw === "function") {
      iter.throw = function (value) {
        if (waiting) {
          waiting = false;
          throw value;
        }

        return pump("throw", value);
      };
    }

    if (typeof inner.return === "function") {
      iter.return = function (value) {
        return pump("return", value);
      };
    }

    return iter;
  };

  babelHelpers.asyncToGenerator = function (fn) {
    return function () {
      var gen = fn.apply(this, arguments);
      return new Promise(function (resolve, reject) {
        function step(key, arg) {
          try {
            var info = gen[key](arg);
            var value = info.value;
          } catch (error) {
            reject(error);
            return;
          }

          if (info.done) {
            resolve(value);
          } else {
            return Promise.resolve(value).then(function (value) {
              step("next", value);
            }, function (err) {
              step("throw", err);
            });
          }
        }

        return step("next");
      });
    };
  };

  babelHelpers.classCallCheck = function (instance, Constructor) {
    if (!(instance instanceof Constructor)) {
      throw new TypeError("Cannot call a class as a function");
    }
  };

  babelHelpers.createClass = function () {
    function defineProperties(target, props) {
      for (var i = 0; i < props.length; i++) {
        var descriptor = props[i];
        descriptor.enumerable = descriptor.enumerable || false;
        descriptor.configurable = true;
        if ("value" in descriptor) descriptor.writable = true;
        Object.defineProperty(target, descriptor.key, descriptor);
      }
    }

    return function (Constructor, protoProps, staticProps) {
      if (protoProps) defineProperties(Constructor.prototype, protoProps);
      if (staticProps) defineProperties(Constructor, staticProps);
      return Constructor;
    };
  }();

  babelHelpers.defineEnumerableProperties = function (obj, descs) {
    for (var key in descs) {
      var desc = descs[key];
      desc.configurable = desc.enumerable = true;
      if ("value" in desc) desc.writable = true;
      Object.defineProperty(obj, key, desc);
    }

    return obj;
  };

  babelHelpers.defaults = function (obj, defaults) {
    var keys = Object.getOwnPropertyNames(defaults);

    for (var i = 0; i < keys.length; i++) {
      var key = keys[i];
      var value = Object.getOwnPropertyDescriptor(defaults, key);

      if (value && value.configurable && obj[key] === undefined) {
        Object.defineProperty(obj, key, value);
      }
    }

    return obj;
  };

  babelHelpers.defineProperty = function (obj, key, value) {
    if (key in obj) {
      Object.defineProperty(obj, key, {
        value: value,
        enumerable: true,
        configurable: true,
        writable: true
      });
    } else {
      obj[key] = value;
    }

    return obj;
  };

  babelHelpers.extends = Object.assign || function (target) {
    for (var i = 1; i < arguments.length; i++) {
      var source = arguments[i];

      for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
          target[key] = source[key];
        }
      }
    }

    return target;
  };

  babelHelpers.get = function get(object, property, receiver) {
    if (object === null) object = Function.prototype;
    var desc = Object.getOwnPropertyDescriptor(object, property);

    if (desc === undefined) {
      var parent = Object.getPrototypeOf(object);

      if (parent === null) {
        return undefined;
      } else {
        return get(parent, property, receiver);
      }
    } else if ("value" in desc) {
      return desc.value;
    } else {
      var getter = desc.get;

      if (getter === undefined) {
        return undefined;
      }

      return getter.call(receiver);
    }
  };

  babelHelpers.inherits = function (subClass, superClass) {
    if (typeof superClass !== "function" && superClass !== null) {
      throw new TypeError("Super expression must either be null or a function, not " + typeof superClass);
    }

    subClass.prototype = Object.create(superClass && superClass.prototype, {
      constructor: {
        value: subClass,
        enumerable: false,
        writable: true,
        configurable: true
      }
    });
    if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass;
  };

  babelHelpers.instanceof = function (left, right) {
    if (right != null && typeof Symbol !== "undefined" && right[Symbol.hasInstance]) {
      return right[Symbol.hasInstance](left);
    } else {
      return left instanceof right;
    }
  };

  babelHelpers.interopRequireDefault = function (obj) {
    return obj && obj.__esModule ? obj : {
      default: obj
    };
  };

  babelHelpers.interopRequireWildcard = function (obj) {
    if (obj && obj.__esModule) {
      return obj;
    } else {
      var newObj = {};

      if (obj != null) {
        for (var key in obj) {
          if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key];
        }
      }

      newObj.default = obj;
      return newObj;
    }
  };

  babelHelpers.newArrowCheck = function (innerThis, boundThis) {
    if (innerThis !== boundThis) {
      throw new TypeError("Cannot instantiate an arrow function");
    }
  };

  babelHelpers.objectDestructuringEmpty = function (obj) {
    if (obj == null) throw new TypeError("Cannot destructure undefined");
  };

  babelHelpers.objectWithoutProperties = function (obj, keys) {
    var target = {};

    for (var i in obj) {
      if (keys.indexOf(i) >= 0) continue;
      if (!Object.prototype.hasOwnProperty.call(obj, i)) continue;
      target[i] = obj[i];
    }

    return target;
  };

  babelHelpers.possibleConstructorReturn = function (self, call) {
    if (!self) {
      throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
    }

    return call && (typeof call === "object" || typeof call === "function") ? call : self;
  };

  babelHelpers.selfGlobal = typeof global === "undefined" ? self : global;

  babelHelpers.set = function set(object, property, value, receiver) {
    var desc = Object.getOwnPropertyDescriptor(object, property);

    if (desc === undefined) {
      var parent = Object.getPrototypeOf(object);

      if (parent !== null) {
        set(parent, property, value, receiver);
      }
    } else if ("value" in desc && desc.writable) {
      desc.value = value;
    } else {
      var setter = desc.set;

      if (setter !== undefined) {
        setter.call(receiver, value);
      }
    }

    return value;
  };

  babelHelpers.slicedToArray = function () {
    function sliceIterator(arr, i) {
      var _arr = [];
      var _n = true;
      var _d = false;
      var _e = undefined;

      try {
        for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) {
          _arr.push(_s.value);

          if (i && _arr.length === i) break;
        }
      } catch (err) {
        _d = true;
        _e = err;
      } finally {
        try {
          if (!_n && _i["return"]) _i["return"]();
        } finally {
          if (_d) throw _e;
        }
      }

      return _arr;
    }

    return function (arr, i) {
      if (Array.isArray(arr)) {
        return arr;
      } else if (Symbol.iterator in Object(arr)) {
        return sliceIterator(arr, i);
      } else {
        throw new TypeError("Invalid attempt to destructure non-iterable instance");
      }
    };
  }();

  babelHelpers.slicedToArrayLoose = function (arr, i) {
    if (Array.isArray(arr)) {
      return arr;
    } else if (Symbol.iterator in Object(arr)) {
      var _arr = [];

      for (var _iterator = arr[Symbol.iterator](), _step; !(_step = _iterator.next()).done;) {
        _arr.push(_step.value);

        if (i && _arr.length === i) break;
      }

      return _arr;
    } else {
      throw new TypeError("Invalid attempt to destructure non-iterable instance");
    }
  };

  babelHelpers.taggedTemplateLiteral = function (strings, raw) {
    return Object.freeze(Object.defineProperties(strings, {
      raw: {
        value: Object.freeze(raw)
      }
    }));
  };

  babelHelpers.taggedTemplateLiteralLoose = function (strings, raw) {
    strings.raw = raw;
    return strings;
  };

  babelHelpers.temporalRef = function (val, name, undef) {
    if (val === undef) {
      throw new ReferenceError(name + " is not defined - temporal dead zone");
    } else {
      return val;
    }
  };

  babelHelpers.temporalUndefined = {};

  babelHelpers.toArray = function (arr) {
    return Array.isArray(arr) ? arr : Array.from(arr);
  };

  babelHelpers.toConsumableArray = function (arr) {
    if (Array.isArray(arr)) {
      for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) arr2[i] = arr[i];

      return arr2;
    } else {
      return Array.from(arr);
    }
  };
})(typeof global === "undefined" ? self : global);

/***/ }),
/* 9 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__topologymanager_topology_layout_index__ = __webpack_require__(10);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__topologymanager_config__ = __webpack_require__(33);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__topologymanager_manager__ = __webpack_require__(38);



window.TopologyORegistry = {
    layouts: {
        test: __WEBPACK_IMPORTED_MODULE_0__topologymanager_topology_layout_index__["a" /* SkydiveTestLayout */]
    },
    config: __WEBPACK_IMPORTED_MODULE_1__topologymanager_config__["a" /* default */],
    manager: __WEBPACK_IMPORTED_MODULE_2__topologymanager_manager__["a" /* default */]
};


/***/ }),
/* 10 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__test_index__ = __webpack_require__(11);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__test_index__["a"]; });



/***/ }),
/* 11 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events__ = __webpack_require__(12);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_events__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__base_ui_index__ = __webpack_require__(4);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__event_observer__ = __webpack_require__(25);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__simplify_index__ = __webpack_require__(28);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__helpers__ = __webpack_require__(1);






class SimplifyDataHandler {
    constructor(data) {
        this.data = data;
    }
    simplify() {
        return new Promise((resolve, reject) => {
            this.mergeSimilarNodes()
                .then(this.flattenSamplesOfTopology.bind(this))
                .then(this.simplifyStructure.bind(this))
                .then(() => {
                resolve(this.data);
            });
        });
    }
    simplifyStructure() {
        return new Promise((resolve, reject) => {
            const strategy = __WEBPACK_IMPORTED_MODULE_3__simplify_index__["a" /* makeSimplificationStrategy */]("structure");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }
    mergeSimilarNodes() {
        return new Promise((resolve, reject) => {
            const strategy = __WEBPACK_IMPORTED_MODULE_3__simplify_index__["a" /* makeSimplificationStrategy */]("merge_similar_nodes");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }
    flattenSamplesOfTopology() {
        return new Promise((resolve, reject) => {
            const strategy = __WEBPACK_IMPORTED_MODULE_3__simplify_index__["a" /* makeSimplificationStrategy */]("flat_samples_of_topology");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }
}
class SkydiveTestLayout {
    constructor(selector) {
        this.eventObserver = new __WEBPACK_IMPORTED_MODULE_2__event_observer__["a" /* default */]();
        this.e = new __WEBPACK_IMPORTED_MODULE_0_events__["EventEmitter"]();
        this.alias = "test";
        this.active = false;
        this.selector = selector;
        this.uiBridge = new __WEBPACK_IMPORTED_MODULE_1__base_ui_index__["c" /* LayoutBridgeUI */](selector);
        this.uiBridge.useEventEmitter(this.e);
        this.e.on('node.selected', (d) => {
            // fix for compatibility with old skydive frontend
            if (!d.metadata) {
                d.metadata = d.Metadata;
            }
            this.notifyHandlers('nodeSelected', d);
        });
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useLayoutUI(new __WEBPACK_IMPORTED_MODULE_1__base_ui_index__["d" /* LayoutUI */](selector));
        this.uiBridge.useNodeUI(new __WEBPACK_IMPORTED_MODULE_1__base_ui_index__["e" /* NodeUI */]());
        this.uiBridge.useGroupUI(new __WEBPACK_IMPORTED_MODULE_1__base_ui_index__["b" /* GroupUI */]());
        this.uiBridge.useEdgeUI(new __WEBPACK_IMPORTED_MODULE_1__base_ui_index__["a" /* EdgeUI */]());
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        this.active = true;
        console.log('current topology graph is ', window.topologyComponent.graph);
    }
    onSyncTopology(data) {
        const existsAnyVm = data.Nodes.some((n) => {
            return n.Metadata.Type === 'libvirt';
        });
        if (!existsAnyVm) {
            this.notifyHandlers('impossibleToBuildRequestedTopology', 'No vms found on this server.');
            return;
        }
        const simplifyHandler = new SimplifyDataHandler({
            Obj: data
        });
        console.log('Before simplification', JSON.stringify(data));
        simplifyHandler.simplify().then((data) => {
            console.log('After simplification', JSON.stringify(data));
            const hostID = data.Obj.Nodes.find((node) => { return node.Metadata.Type == 'host'; }).ID;
            const topologyData = Object(__WEBPACK_IMPORTED_MODULE_4__helpers__["a" /* prepareDataForHierarchyLayout */])(data);
            this.uiBridge.start(topologyData, hostID);
        });
    }
    useConfig(config) {
        this.config = config;
        this.uiBridge.useConfig(this.config);
    }
    remove() {
        this.removeLayout();
    }
    removeLayout() {
        this.active = false;
        this.uiBridge.remove();
        $(this.selector).empty();
        this.e.emit('ui.update');
    }
    addHandler(...args) {
        this.eventObserver.addHandler.apply(this.eventObserver, args);
    }
    removeHandler(...args) {
        this.eventObserver.removeHandler.apply(this.eventObserver, args);
    }
    notifyHandlers(...args) {
        args = [...args];
        args.splice(0, 0, this);
        this.eventObserver.notifyHandlers.apply(this.eventObserver, args);
    }
    autoExpand() {
    }
    zoomIn() {
    }
    zoomOut() {
    }
    zoomFit() {
    }
    collapse() {
    }
    toggleExpandAll() {
    }
    toggleCollapseByLevel() {
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = SkydiveTestLayout;



/***/ }),
/* 12 */
/***/ (function(module, exports) {

// Copyright Joyent, Inc. and other Node contributors.
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to permit
// persons to whom the Software is furnished to do so, subject to the
// following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN
// NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
// OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
// USE OR OTHER DEALINGS IN THE SOFTWARE.

function EventEmitter() {
  this._events = this._events || {};
  this._maxListeners = this._maxListeners || undefined;
}
module.exports = EventEmitter;

// Backwards-compat with node 0.10.x
EventEmitter.EventEmitter = EventEmitter;

EventEmitter.prototype._events = undefined;
EventEmitter.prototype._maxListeners = undefined;

// By default EventEmitters will print a warning if more than 10 listeners are
// added to it. This is a useful default which helps finding memory leaks.
EventEmitter.defaultMaxListeners = 10;

// Obviously not all Emitters should be limited to 10. This function allows
// that to be increased. Set to zero for unlimited.
EventEmitter.prototype.setMaxListeners = function(n) {
  if (!isNumber(n) || n < 0 || isNaN(n))
    throw TypeError('n must be a positive number');
  this._maxListeners = n;
  return this;
};

EventEmitter.prototype.emit = function(type) {
  var er, handler, len, args, i, listeners;

  if (!this._events)
    this._events = {};

  // If there is no 'error' event listener then throw.
  if (type === 'error') {
    if (!this._events.error ||
        (isObject(this._events.error) && !this._events.error.length)) {
      er = arguments[1];
      if (er instanceof Error) {
        throw er; // Unhandled 'error' event
      } else {
        // At least give some kind of context to the user
        var err = new Error('Uncaught, unspecified "error" event. (' + er + ')');
        err.context = er;
        throw err;
      }
    }
  }

  handler = this._events[type];

  if (isUndefined(handler))
    return false;

  if (isFunction(handler)) {
    switch (arguments.length) {
      // fast cases
      case 1:
        handler.call(this);
        break;
      case 2:
        handler.call(this, arguments[1]);
        break;
      case 3:
        handler.call(this, arguments[1], arguments[2]);
        break;
      // slower
      default:
        args = Array.prototype.slice.call(arguments, 1);
        handler.apply(this, args);
    }
  } else if (isObject(handler)) {
    args = Array.prototype.slice.call(arguments, 1);
    listeners = handler.slice();
    len = listeners.length;
    for (i = 0; i < len; i++)
      listeners[i].apply(this, args);
  }

  return true;
};

EventEmitter.prototype.addListener = function(type, listener) {
  var m;

  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  if (!this._events)
    this._events = {};

  // To avoid recursion in the case that type === "newListener"! Before
  // adding it to the listeners, first emit "newListener".
  if (this._events.newListener)
    this.emit('newListener', type,
              isFunction(listener.listener) ?
              listener.listener : listener);

  if (!this._events[type])
    // Optimize the case of one listener. Don't need the extra array object.
    this._events[type] = listener;
  else if (isObject(this._events[type]))
    // If we've already got an array, just append.
    this._events[type].push(listener);
  else
    // Adding the second element, need to change to array.
    this._events[type] = [this._events[type], listener];

  // Check for listener leak
  if (isObject(this._events[type]) && !this._events[type].warned) {
    if (!isUndefined(this._maxListeners)) {
      m = this._maxListeners;
    } else {
      m = EventEmitter.defaultMaxListeners;
    }

    if (m && m > 0 && this._events[type].length > m) {
      this._events[type].warned = true;
      console.error('(node) warning: possible EventEmitter memory ' +
                    'leak detected. %d listeners added. ' +
                    'Use emitter.setMaxListeners() to increase limit.',
                    this._events[type].length);
      if (typeof console.trace === 'function') {
        // not supported in IE 10
        console.trace();
      }
    }
  }

  return this;
};

EventEmitter.prototype.on = EventEmitter.prototype.addListener;

EventEmitter.prototype.once = function(type, listener) {
  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  var fired = false;

  function g() {
    this.removeListener(type, g);

    if (!fired) {
      fired = true;
      listener.apply(this, arguments);
    }
  }

  g.listener = listener;
  this.on(type, g);

  return this;
};

// emits a 'removeListener' event iff the listener was removed
EventEmitter.prototype.removeListener = function(type, listener) {
  var list, position, length, i;

  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  if (!this._events || !this._events[type])
    return this;

  list = this._events[type];
  length = list.length;
  position = -1;

  if (list === listener ||
      (isFunction(list.listener) && list.listener === listener)) {
    delete this._events[type];
    if (this._events.removeListener)
      this.emit('removeListener', type, listener);

  } else if (isObject(list)) {
    for (i = length; i-- > 0;) {
      if (list[i] === listener ||
          (list[i].listener && list[i].listener === listener)) {
        position = i;
        break;
      }
    }

    if (position < 0)
      return this;

    if (list.length === 1) {
      list.length = 0;
      delete this._events[type];
    } else {
      list.splice(position, 1);
    }

    if (this._events.removeListener)
      this.emit('removeListener', type, listener);
  }

  return this;
};

EventEmitter.prototype.removeAllListeners = function(type) {
  var key, listeners;

  if (!this._events)
    return this;

  // not listening for removeListener, no need to emit
  if (!this._events.removeListener) {
    if (arguments.length === 0)
      this._events = {};
    else if (this._events[type])
      delete this._events[type];
    return this;
  }

  // emit removeListener for all listeners on all events
  if (arguments.length === 0) {
    for (key in this._events) {
      if (key === 'removeListener') continue;
      this.removeAllListeners(key);
    }
    this.removeAllListeners('removeListener');
    this._events = {};
    return this;
  }

  listeners = this._events[type];

  if (isFunction(listeners)) {
    this.removeListener(type, listeners);
  } else if (listeners) {
    // LIFO order
    while (listeners.length)
      this.removeListener(type, listeners[listeners.length - 1]);
  }
  delete this._events[type];

  return this;
};

EventEmitter.prototype.listeners = function(type) {
  var ret;
  if (!this._events || !this._events[type])
    ret = [];
  else if (isFunction(this._events[type]))
    ret = [this._events[type]];
  else
    ret = this._events[type].slice();
  return ret;
};

EventEmitter.prototype.listenerCount = function(type) {
  if (this._events) {
    var evlistener = this._events[type];

    if (isFunction(evlistener))
      return 1;
    else if (evlistener)
      return evlistener.length;
  }
  return 0;
};

EventEmitter.listenerCount = function(emitter, type) {
  return emitter.listenerCount(type);
};

function isFunction(arg) {
  return typeof arg === 'function';
}

function isNumber(arg) {
  return typeof arg === 'number';
}

function isObject(arg) {
  return typeof arg === 'object' && arg !== null;
}

function isUndefined(arg) {
  return arg === void 0;
}


/***/ }),
/* 13 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class NodeUI {
    useLayoutContext(layoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot(g, nodes) {
        this.root = g;
        this.g = g.append("g").attr('class', 'nodes').selectAll("circle.node");
    }
    tick() {
        this.g.attr("transform", function (d) {
            if (d.px && d.py) {
                return "translate(" + d.px + "," + d.py + ")";
            }
            else {
                return "translate(" + d.x + "," + d.y + ")";
            }
        });
    }
    update(nodes) {
        // Update the nodes…
        this.g = this.g.data(nodes, function (d) { return d.id; });
        this.g.exit().remove();
        const nodeEnter = this.g.enter()
            .append("g")
            .attr("class", (d) => {
            const classParts = ['node'];
            classParts.push(d.Metadata.Type);
            return classParts.join(' ');
        })
            .attr("id", function (d) { return "node-" + d.id; })
            .on("click", this.onNodeClick.bind(this))
            .on("mousedown", this.onMouseDown.bind(this))
            .call(window.d3.drag());
        nodeEnter.append("circle")
            .attr("r", this.layoutContext.config.getValue('node.radius'));
        // node picto
        nodeEnter.append("image")
            .attr("id", function (d) { return "node-img-" + d.id; })
            .attr("class", "picto")
            .attr("x", -12)
            .attr("y", -12)
            .attr("width", "24")
            .attr("height", "24")
            .attr("xlink:href", this.nodeImg.bind(this));
        // node title
        nodeEnter.append("text")
            .attr("dx", (d) => {
            return this.layoutContext.config.getValue('node.size', d) + 15;
        })
            .attr("dy", 10)
            .attr("class", "node-label")
            .text((d) => { return this.layoutContext.config.getValue('node.title', d); });
        // node type label
        nodeEnter.append("text")
            .attr("dx", (d) => { return this.layoutContext.config.getValue('node.typeLabelXShift', d); })
            .attr("dy", (d) => { return this.layoutContext.config.getValue('node.typeLabelYShift', d); })
            .attr("class", "type-label")
            .text((d) => { return this.layoutContext.config.getValue('node.typeLabel', d); });
        this.g = nodeEnter.merge(this.g);
    }
    nodeImg(d) {
        const t = d.Metadata.Type || "default";
        if (this.layoutContext.config.getValue('typesWithTextLabels').indexOf(t) !== -1)
            return null;
        return (t in window.nodeImgMap) ? window.nodeImgMap[t] : window.nodeImgMap["default"];
    }
    onNodeClick(d) {
        if (this.selectedNode === d)
            return;
        if (this.selectedNode)
            this.unselectNode(this.selectedNode);
        this.selectNode(d);
        // this.layoutContext.e.emit('node.selected', d);
    }
    onMouseDown(d) {
        this.onNodeClick(d);
    }
    selectNode(d) {
        console.log(d.id);
        var circle = this.root.select("#node-" + d.id)
            .select('circle');
        console.log(this.g, circle, this.g.select("#node-" + d.id));
        circle.transition().duration(500).attr('r', +circle.attr('r') + this.layoutContext.config.getValue('node.clickSizeDiff'));
        d.selected = true;
        this.selectedNode = d;
        this.updateGraph(d);
    }
    unselectNode(d) {
        var circle = this.root.select("#node-" + d.id)
            .select('circle');
        if (!circle)
            return;
        circle.transition().duration(500).attr('r', circle ? +circle.attr('r') - this.layoutContext.config.getValue('node.clickSizeDiff') : 0);
        d.selected = false;
        this.selectedNode = null;
        this.updateGraph(d);
    }
    updateGraph(d) {
        const nodeLabel = this.root.select("#node-" + d.id)
            .select('text.node-label');
        nodeLabel
            .attr("filter", d.selected ? "url(#selectedNodeFilter)" : null)
            .style("fill", this.layoutContext.config.getValue('node.labelColor'))
            .text((d) => { return this.layoutContext.config.getValue('node.title', d); });
        this.root.selectAll("g.node").sort(function (a, b) {
            if (a.id === d.id)
                return d.selected ? 1 : -1;
            if (b.id === d.id)
                return d.selected ? -1 : -1;
            return -1;
        });
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = NodeUI;



/***/ }),
/* 14 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class LayoutUI {
    constructor(selector) {
        this.selector = selector;
    }
    useLayoutContext(layoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot() {
        this.width = $(this.selector).width() - 20;
        this.height = $(window).height() - (window.$(this.selector).offset() && window.$(this.selector).offset().top || 0);
        this.svg = window.d3.select(this.selector).append("svg");
        this.zoom = window.d3.zoom()
            .on("zoom", this.zoomed.bind(this));
        this.svg
            .attr("width", this.width)
            .attr("height", this.height)
            .call(this.zoom);
        this.simulation = window.d3.forceSimulation();
        this.simulation.nodes([]);
        this.simulation.on("tick", () => {
            this.layoutContext.e.emit('ui.tick');
        });
        const filter = this.svg.append("filter").attr("x", 0).attr("y", 0)
            .attr("width", 1).attr("height", 1).attr("id", "selectedNodeFilter");
        filter.append("feFlood").attr("flood-color", "#ccc");
        filter.append("feComposite").attr("in", "SourceGraphic");
        this.g = this.svg.append("g").attr("transform", function () { return "translate(0,33)"; });
        ;
    }
    start() {
    }
    restartsimulation() {
    }
    zoomed() {
        this.g.attr("transform", window.d3.event.transform);
    }
    zoomIn() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 1.1);
    }
    zoomOut() {
        this.svg.transition().duration(500).call(this.zoom.scaleBy, 0.9);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutUI;



/***/ }),
/* 15 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class GroupUI {
    useLayoutContext(layoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot(g) {
        this.g = g.append("g").attr('class', 'groups').selectAll(".group");
    }
    get root() {
        return this.g;
    }
    tick() {
    }
    update() {
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = GroupUI;



/***/ }),
/* 16 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class EdgeUI {
    constructor() {
        this.nodeIDToNode = {};
    }
    useLayoutContext(layoutContext) {
        this.layoutContext = layoutContext;
    }
    createRoot(g, nodes) {
        this.root = g;
        this.nodeIDToNode = nodes.reduce((accum, n) => {
            accum[n.ID] = n;
            return accum;
        }, {});
        this.gLink = g.append("g").attr('class', 'links').selectAll("line.link");
    }
    tick() {
        this.gLink.attr("d", (d) => {
            if (d.generateD3Path) {
                return d.generateD3Path().path;
            }
            // return '';
            let dModification = 'M ';
            let source = this.nodeIDToNode[d.source.data.ID];
            let target = this.nodeIDToNode[d.target.data.ID];
            // @todo  remove
            if (!source || !target) {
                console.log('Absent source or target.', d);
                return;
            }
            if (target.py && source.py && target.py < source.py) {
                const tmp = target;
                target = source;
                source = tmp;
            }
            else if (target.y < source.y) {
                const tmp = target;
                target = source;
                source = tmp;
            }
            const verticalIncrement = this.layoutContext.config.getValue('node.height') / 2;
            if (source.px && source.py) {
                dModification += source.px + " " + (source.py + verticalIncrement);
            }
            else {
                dModification += source.x + " " + (source.y + verticalIncrement);
            }
            dModification += " L ";
            if (target.px && target.py) {
                dModification += target.px + " " + (target.py - verticalIncrement);
            }
            else {
                dModification += target.x + " " + (target.y - verticalIncrement);
            }
            return dModification;
        });
    }
    update(links) {
        // Update the links…
        this.gLink = this.gLink.data(links, function (d) { return d.id; });
        this.gLink.exit().remove();
        const linkEnter = this.gLink.enter()
            .append("path")
            .attr("id", function (d) { return d.generateD3Path ? Math.random() : "link-" + d.source.data.ID + "-" + d.target.data.ID; })
            .attr("class", (d) => { return this.layoutContext.config.getValue('link.className', d); })
            .style("stroke", (d) => { return this.layoutContext.config.getValue('link.color', d); });
        this.gLink = linkEnter.merge(this.gLink);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = EdgeUI;



/***/ }),
/* 17 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__layout_context__ = __webpack_require__(18);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__common_index__ = __webpack_require__(19);


class LayoutBridgeUI {
    constructor(selector) {
        this.initialized = false;
        this.selector = selector;
    }
    useEventEmitter(e) {
        this.e = e;
    }
    useNodeUI(nodeUI) {
        this.nodeUI = nodeUI;
    }
    useGroupUI(groupUI) {
        this.groupUI = groupUI;
    }
    useEdgeUI(edgeUI) {
        this.edgeUI = edgeUI;
    }
    useConfig(config) {
        this.config = config;
    }
    useLayoutUI(layoutUI) {
        this.layoutUI = layoutUI;
    }
    start(data, hostID) {
        this.initialized = false;
        this.layoutUI.useLayoutContext(this.layoutContext);
        this.layoutUI.createRoot();
        this.groupUI.useLayoutContext(this.layoutContext);
        this.groupUI.createRoot(this.layoutUI.g);
        this.layoutContext.subscribeToEvent('ui.tick', this.tick.bind(this));
        this.layoutContext.subscribeToEvent('ui.update', this.update.bind(this));
        const flattener = new __WEBPACK_IMPORTED_MODULE_1__common_index__["a" /* FlattenNodesForHierarchy */](this.config);
        let nodes = flattener.flattenForWidthAndHeight(hostID, data, this.layoutUI.width, this.layoutUI.height);
        const nodeIDToNode = nodes.reduce((acc, node) => {
            acc[node.ID] = node;
            return acc;
        }, {});
        nodes = Object.keys(nodeIDToNode).map(nodeId => nodeIDToNode[nodeId]);
        let links = window.d3.hierarchy(data).links();
        const linkManager = new __WEBPACK_IMPORTED_MODULE_1__common_index__["c" /* LinkManager */]();
        console.log('Before links optimization', links.length);
        links = linkManager.optimizeLinks(links);
        links.forEach((link) => {
            if (!(link instanceof __WEBPACK_IMPORTED_MODULE_1__common_index__["b" /* LineWithCommonBus */])) {
                return;
            }
            link.graph = flattener.graph;
        });
        console.log('After links optimization', links.length);
        console.log(links);
        console.log(nodes);
        console.log(nodeIDToNode);
        this.nodes = nodes;
        this.links = links;
        this.edgeUI.useLayoutContext(this.layoutContext);
        this.edgeUI.createRoot(this.layoutUI.g, nodes);
        this.nodeUI.useLayoutContext(this.layoutContext);
        this.nodeUI.createRoot(this.layoutUI.g, nodes);
        this.layoutUI.start();
        this.initialized = true;
        this.e.emit('ui.update');
    }
    remove() {
    }
    get layoutContext() {
        const context = new __WEBPACK_IMPORTED_MODULE_0__layout_context__["a" /* default */]();
        context.e = this.e;
        context.config = this.config;
        return context;
    }
    tick() {
        this.edgeUI.tick();
        this.nodeUI.tick();
        this.groupUI.tick();
    }
    update() {
        if (!this.initialized) {
            return;
        }
        this.nodeUI.update(this.nodes);
        this.edgeUI.update(this.links);
        this.groupUI.update();
        this.layoutUI.restartsimulation();
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutBridgeUI;



/***/ }),
/* 18 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class LayoutContext {
    subscribeToEvent(eventName, cb) {
        this.e.on(eventName, cb);
    }
    unsubscribeFromEvent(eventName, cb) {
        if (cb) {
            this.e.removeListener(eventName, cb);
        }
        else {
            this.e.removeAllListeners(eventName);
        }
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutContext;



/***/ }),
/* 19 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__flattener_for_hierarchy__ = __webpack_require__(20);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__flattener_for_hierarchy__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__link_manager__ = __webpack_require__(24);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return __WEBPACK_IMPORTED_MODULE_1__link_manager__["b"]; });
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return __WEBPACK_IMPORTED_MODULE_1__link_manager__["a"]; });




/***/ }),
/* 20 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__algorithms_index__ = __webpack_require__(21);

class FlattenNodesForHierarchyAspect {
    constructor(config) {
        this.graph = new __WEBPACK_IMPORTED_MODULE_0__algorithms_index__["a" /* HierarchicalGraph */]();
        this.graph = new __WEBPACK_IMPORTED_MODULE_0__algorithms_index__["a" /* HierarchicalGraph */]();
    }
    recurseFlattening(graph, vertical, node, parent, depth) {
        node.childrenIds && node.childrenIds.delete && node.childrenIds.delete(this.hostID);
        node.depth = depth;
        node.horizontal = depth;
        node.fixed = true;
        node.width = 33;
        graph.addVertex(node);
        if (node.children) {
            node.children.forEach((children) => {
                this.recurseFlattening(graph, vertical, children, node, depth + 1);
            });
        }
    }
    flattenForWidthAndHeight(hostID, data, width, height) {
        this.hostID = hostID;
        this.graph.width = width;
        this.graph.height = height;
        data.children.forEach((nodeVm, vertical) => {
            const graph = Object(__WEBPACK_IMPORTED_MODULE_0__algorithms_index__["b" /* makeGraph */])();
            this.graph.children.addGraph(graph);
            this.recurseFlattening(graph, vertical, nodeVm, null, 0);
        });
        this.graph.children.mergeGraphsWithCommonVertexes();
        console.log('number of graphs', this.graph.children.graphs.length);
        this.graph.balance();
        const nodes = this.graph.children.getVertexes();
        console.log('NODES AFTER GRAPH BALANCING', nodes);
        nodes.sort((nodeA, nodeB) => {
            if (nodeA.depth != nodeB.depth) {
                return nodeA.depth - nodeB.depth;
            }
            return nodeA.x - nodeB.x;
        });
        return nodes;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = FlattenNodesForHierarchyAspect;



/***/ }),
/* 21 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["b"] = makeGraph;
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__hierarchical_graph__ = __webpack_require__(22);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__hierarchical_graph__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__graph_registry__ = __webpack_require__(2);
/* unused harmony reexport Graph */



function makeGraph() {
    return new __WEBPACK_IMPORTED_MODULE_1__graph_registry__["a" /* Graph */]();
}


/***/ }),
/* 22 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__graph_registry__ = __webpack_require__(2);

class HierarchicalGraph {
    constructor() {
        this.nodeWidth = 150;
        this.children = new __WEBPACK_IMPORTED_MODULE_0__graph_registry__["b" /* default */]();
    }
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
        this.children.graphs.forEach((g, i) => {
            g.calculateCoordinatesForVertices(xMultiplier, yMultiplier, i === 0 ? this.nodeWidth / 2 : this.children.graphs[i - 1].getLastXCoord() + this.nodeWidth, 0);
        });
    }
    isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds) {
        return this.children.graphs.some((g) => {
            return g.isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds);
        });
    }
    suggestBetterFreeXCoord(x, minHorizontal, maxHorizontal, ignoreVIds) {
        const graph = this.children.graphs.find((g) => {
            return g.isXCoordBusy(x, minHorizontal, maxHorizontal, ignoreVIds);
        });
        return graph.suggestBetterFreeXCoord(x, minHorizontal, maxHorizontal);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = HierarchicalGraph;



/***/ }),
/* 23 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["a"] = range;
function range(size, startAt = 0, defaultValue = null) {
    return [...Array(size).keys()].map(i => defaultValue !== null ? defaultValue : i + startAt);
}


/***/ }),
/* 24 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class LinkManager {
    constructor() {
        this.dataKeeper = {};
    }
    removeTopHierarchyLink(links) {
        return links.filter((link) => {
            return !(link.target.data.Type === "tophierarchy" || link.source.data.Type === "tophierarchy");
        });
    }
    removeLinksWithTheSameSourceAndTarget(links) {
        const currentlyExistingLinks = new Set([]);
        return links.filter((link) => {
            const linkId = link.target.data.ID + "-" + link.source.data.ID;
            if (currentlyExistingLinks.has(linkId)) {
                return false;
            }
            currentlyExistingLinks.add(linkId);
            return true;
        });
    }
    findCommonBuses(links) {
        const busesListToAdd = [];
        let source, target;
        const sourceToTargets = {};
        const targetToSources = {};
        links.forEach((link) => {
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target;
                target = link.source;
            }
            else {
                source = link.source;
                target = link.target;
            }
            if (!sourceToTargets[source.data.ID]) {
                sourceToTargets[source.data.ID] = new Set([]);
            }
            sourceToTargets[source.data.ID].add(target.data.ID);
            if (!targetToSources[target.data.ID]) {
                targetToSources[target.data.ID] = new Set([]);
            }
            targetToSources[target.data.ID].add(source.data.ID);
        });
        links.forEach((link) => {
            if (link instanceof LineWithCommonBus) {
                return;
            }
            let bus;
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target;
                target = link.source;
            }
            else {
                source = link.source;
                target = link.target;
            }
            const targets = sourceToTargets[source.data.ID];
            const sources = targetToSources[target.data.ID];
            if (targets.size > 1) {
                if (!source.data.buses) {
                    source.data.buses = new BusesList();
                }
                bus = source.data.buses.findByHorizontal(source.data, target.data);
                if (!bus) {
                    bus = new LineWithCommonBus(source.data);
                    source.data.buses.addBus(bus);
                    busesListToAdd.push(bus);
                }
                bus.addTarget(target.data);
            }
            else if (sources.size > 1) {
                if (!target.data.buses) {
                    target.data.buses = new BusesList();
                }
                bus = target.data.buses.findByHorizontal(target.data, source.data);
                if (!bus) {
                    bus = new LineWithCommonBus(target.data);
                    target.data.buses.addBus(bus);
                    busesListToAdd.push(bus);
                }
                bus.addTarget(source.data);
            }
        });
        links = links.filter((link) => {
            if (link instanceof LineWithCommonBus) {
                return true;
            }
            let buses;
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target.data;
                target = link.source.data;
            }
            else {
                source = link.source.data;
                target = link.target.data;
            }
            const targets = sourceToTargets[source.ID];
            const sources = targetToSources[target.ID];
            if (targets.size > 1) {
                buses = source.buses;
            }
            else if (sources.size > 1) {
                buses = target.buses;
            }
            return !buses;
        });
        const matrixBusHandler = new MatrixBusHandler();
        busesListToAdd.forEach((bus) => {
            matrixBusHandler.addBus(bus);
        });
        busesListToAdd.forEach((bus) => {
            bus.matrixHandler = matrixBusHandler;
            bus.targets.sort((a, b) => {
                return a.vertical - b.vertical;
            });
            links.push(bus);
        });
        return links;
    }
    optimizeLinks(links) {
        links = this.removeTopHierarchyLink(links);
        links = this.removeLinksWithTheSameSourceAndTarget(links);
        links = this.findCommonBuses(links);
        // links = links.filter((link: any) => {
        //     if (!(link instanceof LineWithCommonBus)) {
        //         return true;
        //     }
        //     if (link.source.Name === 'sriov-lb-vm1-4642133b') {
        //         return true;
        //     }
        //     return false;
        // })
        return links;
    }
}
/* harmony export (immutable) */ __webpack_exports__["b"] = LinkManager;

class MatrixBusHandler {
    constructor() {
        this.all_buses = [];
        this.buses = {};
        this.pullOfColors = new Set([
            '#000080',
            '#FF00FF',
            '#800080',
            '#00FF00',
            '#008000',
            '#00FFFF',
            '#0000FF'
        ]);
        this.usedColorsPerHorizontal = new Set([]);
        this.yOffsets = {};
        this.calculatedBusesPerHorizontal = false;
    }
    addBus(bus) {
        this.all_buses.push(bus);
    }
    chooseColorForHorizontal(horizontal) {
        this.calculateBusesPerHorizontal();
        const leftColors = new Set([...this.pullOfColors].filter(x => !this.usedColorsPerHorizontal.has(x)));
        if (leftColors.size) {
            const color = leftColors.values().next().value;
            this.usedColorsPerHorizontal.add(color);
            return color;
        }
        return '#111';
    }
    calculateBusesPerHorizontal() {
        if (this.calculatedBusesPerHorizontal) {
            return;
        }
        this.all_buses.forEach((bus) => {
            if (!this.buses[bus.horizontal]) {
                this.buses[bus.horizontal] = [];
            }
            this.buses[bus.horizontal].push(bus);
        });
        this.calculatedBusesPerHorizontal = true;
    }
    getNumberOfBusesPerHorizontal(horizontal, node) {
        this.calculateBusesPerHorizontal();
        if (node && this.buses[horizontal]) {
            return this.buses[horizontal].length;
            // const foundBusesWithNode = this.buses[horizontal].filter((b: LineWithCommonBus) => {
            //     if (b.source.ID === node.ID) {
            //         return true;
            //     }
            //     return false;
            // });
            // return foundBusesWithNode.length ? foundBusesWithNode.length : 1;
        }
        return this.buses[horizontal] ? this.buses[horizontal].length : 1;
    }
    getCurrentYOffsetPerHorizontal(width, horizontal) {
        this.calculateBusesPerHorizontal();
        if (!this.yOffsets[horizontal]) {
            this.yOffsets[horizontal] = 0;
        }
        this.yOffsets[horizontal] += width / 2 / this.getNumberOfBusesPerHorizontal(horizontal);
        return this.yOffsets[horizontal];
    }
}
/* unused harmony export MatrixBusHandler */

function generateConnectorToNode(node, connectorNodeCoordinates, generalBusY) {
    if (connectorNodeCoordinates.startsAtX === connectorNodeCoordinates.stopsAtX) {
        const parts = ['M ' + connectorNodeCoordinates.stopsAtX.toString() + ' ' + connectorNodeCoordinates.stopsAtY.toString() + ' L ' + connectorNodeCoordinates.startsAtX.toString() + ' ' + connectorNodeCoordinates.startsAtY.toString()];
        if (node.x !== connectorNodeCoordinates.startsAtX) {
            let nodeYCoord;
            if (node.y > connectorNodeCoordinates.startsAtY) {
                nodeYCoord = node.y - node.width;
            }
            else {
                nodeYCoord = node.y + node.width;
            }
            if (connectorNodeCoordinates.startsAtY > nodeYCoord) {
                parts.push('M ' + connectorNodeCoordinates.stopsAtX.toString() + ' ' + connectorNodeCoordinates.stopsAtY.toString() + ' ');
            }
            parts.push('L ' + node.x.toString());
            parts.push(nodeYCoord);
        }
        return parts.join(' ') + ' ';
    }
    else {
        const parts = ['M'];
        parts.push(node.x);
        if (node.y > connectorNodeCoordinates.startsAtY) {
            parts.push(node.y - node.width);
        }
        else {
            parts.push(node.y + node.width);
        }
        parts.push('L');
        if (connectorNodeCoordinates.startsAtX !== node.x) {
            parts.push(connectorNodeCoordinates.stopsAtX);
        }
        else {
            parts.push(connectorNodeCoordinates.startsAtX);
        }
        parts.push(generalBusY);
        parts.push('L');
        if (connectorNodeCoordinates.startsAtX !== node.x) {
            parts.push(connectorNodeCoordinates.startsAtX);
        }
        else {
            parts.push(connectorNodeCoordinates.stopsAtX);
        }
        parts.push(generalBusY);
        return parts.join(' ') + ' ';
    }
}
class LineWithCommonBus {
    constructor(source) {
        this.targets = [];
        this.source = source;
    }
    get horizontal() {
        if (this.targets[0].horizontal > this.source.horizontal) {
            return this.source.horizontal + 1;
        }
        else {
            return this.source.horizontal;
        }
    }
    addTarget(target) {
        if (!target.targetbuses) {
            target.targetbuses = new BusesList();
        }
        target.targetbuses.addBus(this);
        this.targets.push(target);
    }
    generateD3Path() {
        if (this.generatedD3Path) {
            return this.generatedD3Path;
        }
        const currentYOffset = this.matrixHandler.getCurrentYOffsetPerHorizontal(this.source.width, this.horizontal);
        const targetConnectors = [];
        let generalBusY;
        if (this.source.y > this.targets[0].y) {
            generalBusY = this.source.y - this.source.width - currentYOffset;
        }
        else {
            generalBusY = this.source.y + this.source.width + currentYOffset;
        }
        const coordinatesForTargets = [];
        const nodesInThisBus = this.targets.map(t => t.ID);
        nodesInThisBus.push(this.source.ID);
        this.targets.forEach((target) => {
            let xOffset = 0;
            if (this.source.y > target.y) {
                if (target.busyXCoordAtBottom === undefined) {
                    target.busyXCoordAtBottom = 0;
                }
                else {
                    target.busyXCoordAtBottom += this.source.width / 2 / 5;
                }
                xOffset = target.busyXCoordAtBottom;
            }
            else {
                if (target.busyXCoordAtTop === undefined) {
                    target.busyXCoordAtTop = 0;
                }
                else {
                    target.busyXCoordAtTop += this.source.width / 2 / 5;
                }
                xOffset = target.busyXCoordAtTop;
            }
            let startFromX = target.x - xOffset;
            const minHorizontal = Math.min(target.horizontal, this.source.horizontal);
            const maxHorizontal = Math.max(target.horizontal, this.source.horizontal);
            if (this.graph.isXCoordBusy(startFromX, minHorizontal, maxHorizontal, nodesInThisBus)) {
                startFromX = this.graph.suggestBetterFreeXCoord(startFromX, minHorizontal, maxHorizontal, nodesInThisBus);
            }
            let startFromY;
            if (this.source.y > target.y) {
                startFromY = target.y + target.width;
            }
            else {
                startFromY = target.y - target.width;
            }
            const stopAtX = startFromX;
            const stopAtY = generalBusY;
            coordinatesForTargets.push([startFromX, startFromY, stopAtX, stopAtY, target]);
        });
        coordinatesForTargets.sort((a, b) => {
            return a[0] - b[0];
        });
        let connectorToSourceStartsAtY, connectorToSourceStopsAtY;
        if (this.source.y > this.targets[0].y) {
            connectorToSourceStartsAtY = generalBusY;
            connectorToSourceStopsAtY = this.source.y - this.source.width;
        }
        else {
            connectorToSourceStartsAtY = generalBusY;
            connectorToSourceStopsAtY = this.source.y + this.source.width;
        }
        const generalBusStartsFromX = coordinatesForTargets[0][0];
        const generalBusEndsAtX = coordinatesForTargets[coordinatesForTargets.length - 1][0];
        let connectorToSourceStartsAtX;
        let connectorToSourceStopsAtX;
        if (this.source.x > this.targets[0].x && this.source.x < this.targets[this.targets.length - 1].x) {
            connectorToSourceStartsAtX = this.source.x - this.source.width + this.source.width / this.matrixHandler.getNumberOfBusesPerHorizontal(this.targets[0].horizontal, this.source);
            connectorToSourceStopsAtX = this.source.x - this.source.width + this.source.width / this.matrixHandler.getNumberOfBusesPerHorizontal(this.targets[0].horizontal, this.source);
        }
        else if (this.source.x > this.targets[0].x) {
            connectorToSourceStartsAtX = this.source.x;
            connectorToSourceStopsAtX = generalBusEndsAtX;
        }
        else {
            connectorToSourceStartsAtX = generalBusStartsFromX;
            connectorToSourceStopsAtX = this.source.x;
        }
        coordinatesForTargets.forEach((coords, targetNumber) => {
            const connectorToTarget = generateConnectorToNode(coords[4], {
                startsAtX: coords[0],
                startsAtY: coords[1],
                stopsAtX: coords[2],
                stopsAtY: coords[3]
            }, generalBusY);
            targetConnectors.push(connectorToTarget);
        });
        const generalBus = 'M ' + generalBusStartsFromX.toString() + ' ' + generalBusY.toString() + ' L ' + generalBusEndsAtX.toString() + ' ' + generalBusY.toString();
        const connectorToSource = generateConnectorToNode(this.source, {
            startsAtX: connectorToSourceStartsAtX,
            startsAtY: connectorToSourceStartsAtY,
            stopsAtX: connectorToSourceStopsAtX,
            stopsAtY: connectorToSourceStopsAtY
        }, generalBusY);
        this.generatedD3Path = {
            path: targetConnectors.join(' ') + ' ' + generalBus + ' ' + connectorToSource,
            color: this.matrixHandler.chooseColorForHorizontal(this.horizontal)
        };
        return this.generatedD3Path;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LineWithCommonBus;

class BusesList {
    constructor() {
        this.buses = [];
    }
    findByHorizontal(source, target) {
        const horizontal = source.horizontal > target.horizontal ? source.horizontal : source.horizontal + 1;
        return this.buses.find((bus) => {
            const matches = horizontal === bus.horizontal;
            return matches;
        });
    }
    addBus(bus) {
        this.buses.push(bus);
    }
    getLength() {
        return this.buses.length;
    }
    getBusesOnHorizontal(horizontal) {
        return this.buses.filter((bus) => { return bus.horizontal === horizontal; });
    }
}
/* unused harmony export BusesList */



/***/ }),
/* 25 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
const upperCaseFirst = __webpack_require__(26);
class EventObserver {
    constructor() {
        this.handlers = [];
    }
    addHandler(handler) {
        this.handlers.push(handler);
    }
    removeHandler(handler) {
        var index = this.handlers.indexOf(handler);
        if (index > -1) {
            this.handlers.splice(index, 1);
        }
    }
    notifyHandlers(obj, ev, v1, v2) {
        var i, h;
        for (i = this.handlers.length - 1; i >= 0; i--) {
            h = this.handlers[i];
            try {
                var callback = h["on" + upperCaseFirst(ev)];
                if (callback) {
                    callback.bind(h)(v1, v2);
                }
            }
            catch (e) {
                console.log(e);
            }
        }
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = EventObserver;



/***/ }),
/* 26 */
/***/ (function(module, exports, __webpack_require__) {

var upperCase = __webpack_require__(27)

/**
 * Upper case the first character of a string.
 *
 * @param  {String} str
 * @return {String}
 */
module.exports = function (str, locale) {
  if (str == null) {
    return ''
  }

  str = String(str)

  return upperCase(str.charAt(0), locale) + str.substr(1)
}


/***/ }),
/* 27 */
/***/ (function(module, exports) {

/**
 * Special language-specific overrides.
 *
 * Source: ftp://ftp.unicode.org/Public/UCD/latest/ucd/SpecialCasing.txt
 *
 * @type {Object}
 */
var LANGUAGES = {
  tr: {
    regexp: /[\u0069]/g,
    map: {
      '\u0069': '\u0130'
    }
  },
  az: {
    regexp: /[\u0069]/g,
    map: {
      '\u0069': '\u0130'
    }
  },
  lt: {
    regexp: /[\u0069\u006A\u012F]\u0307|\u0069\u0307[\u0300\u0301\u0303]/g,
    map: {
      '\u0069\u0307': '\u0049',
      '\u006A\u0307': '\u004A',
      '\u012F\u0307': '\u012E',
      '\u0069\u0307\u0300': '\u00CC',
      '\u0069\u0307\u0301': '\u00CD',
      '\u0069\u0307\u0303': '\u0128'
    }
  }
}

/**
 * Upper case a string.
 *
 * @param  {String} str
 * @return {String}
 */
module.exports = function (str, locale) {
  var lang = LANGUAGES[locale]

  str = str == null ? '' : String(str)

  if (lang) {
    str = str.replace(lang.regexp, function (m) { return lang.map[m] })
  }

  return str.toUpperCase()
}


/***/ }),
/* 28 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["a"] = makeSimplificationStrategy;
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__structure__ = __webpack_require__(29);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__merge_nodes_with_the_same_name__ = __webpack_require__(31);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__flatten_parts_of_topology__ = __webpack_require__(32);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__transformation__ = __webpack_require__(0);




const TransformationRegistry = __WEBPACK_IMPORTED_MODULE_3__transformation__["a" /* default */];
/* unused harmony export TransformationRegistry */

function makeSimplificationStrategy(strategyType) {
    const typeToStrategy = {
        structure: __WEBPACK_IMPORTED_MODULE_0__structure__["a" /* default */],
        merge_similar_nodes: __WEBPACK_IMPORTED_MODULE_1__merge_nodes_with_the_same_name__["a" /* default */],
        flat_samples_of_topology: __WEBPACK_IMPORTED_MODULE_2__flatten_parts_of_topology__["a" /* default */]
    };
    return new typeToStrategy[strategyType]();
}


/***/ }),
/* 29 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__transformation__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__base__ = __webpack_require__(3);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__helpers__ = __webpack_require__(1);



class StructureSimplificationStrategy extends __WEBPACK_IMPORTED_MODULE_1__base__["a" /* default */] {
    findDeadNodes() {
        let vmRelatedNodes = new Set([]);
        let nodes = [];
        const nodeTypesToCheck = ["libvirt"];
        const skipHosts = new Set([]);
        // find initial list of nodes, actually, its vm nodes
        Object.keys(this.data).forEach((nodeId) => {
            if (nodeTypesToCheck.indexOf(this.data[nodeId].Type) === -1) {
                return;
            }
            nodes.push(nodeId);
        });
        let i = 0;
        while (true) {
            const nodeId = nodes[i];
            const nodeData = this.data[nodeId];
            if (!nodeData) {
                break;
            }
            let goThroughNodes = [];
            try {
                if (nodeData.Type === "host") {
                    continue;
                }
                // sometimes nodes are not connected directly but
                // connected through layer2 relations, its important
                // for us as well, for instance, here is a typical
                // connection from skydive data structure
                // {
                // "ID":"08883be7-f8be-583d-4bf8-fcd3e611a05f",
                // "Metadata":{
                // "RelationType":"layer2"
                // },
                // "Parent":"ec13216d-af7e-42df-5efe-a478733545de",
                // "Child":"71ace516-0ade-432f-4531-4b5e2a828ced",
                // "Host":"DELL2",
                // "CreatedAt":1524827146311,
                // "UpdatedAt":1524827146311
                // },
                // it needs to be treated as well to make sure we
                // don't lose any information from topology
                if (nodeData.layers.size) {
                    goThroughNodes = goThroughNodes.concat([...nodeData.layers]);
                }
                // handle list of children of specific node
                if (nodeData.children.size) {
                    goThroughNodes = goThroughNodes.concat([...nodeData.children]);
                }
                // handle list of parent of specific node
                if (nodeData.parent.size) {
                    goThroughNodes = goThroughNodes.concat([...nodeData.parent]);
                }
            }
            finally {
                goThroughNodes = goThroughNodes.filter((nodeId1) => {
                    return nodes.indexOf(nodeId1) === -1;
                });
                if (!goThroughNodes.length && (nodes.length - 1) == i) {
                    break;
                }
                const uniqueNodes = new Set(goThroughNodes);
                let uniqueNodes1 = [...uniqueNodes];
                nodes = nodes.concat(uniqueNodes1);
                ++i;
            }
        }
        return Object.keys(this.data).filter((nodeId) => {
            return nodes.indexOf(nodeId) === -1;
        });
    }
    simplifyInitialTopology(dataFormat, data, deadNodes) {
        const transformationHandler = __WEBPACK_IMPORTED_MODULE_0__transformation__["a" /* default */].getTransformationWay(dataFormat);
        return transformationHandler(data, deadNodes);
    }
    simplifyData(dataFormat, data) {
        this.data = Object(__WEBPACK_IMPORTED_MODULE_2__helpers__["b" /* transformSkydiveDataForSimplification */])(data);
        const deadNodes = this.findDeadNodes();
        const newTopology = this.simplifyInitialTopology(dataFormat, data, deadNodes);
        return newTopology;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = StructureSimplificationStrategy;



/***/ }),
/* 30 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";

module.exports = function (x) {
	var type = typeof x;
	return x !== null && (type === 'object' || type === 'function');
};


/***/ }),
/* 31 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__transformation__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__base__ = __webpack_require__(3);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__helpers__ = __webpack_require__(1);



const patternsToBeSimplified = [
    {
        "nodes": {
            "openvswitchpatch": {
                "main": true,
                "Metadata": {
                    "Driver": "openvswitch",
                    "Type": "patch"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportopenvswitchpatchlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "openvswitchpatch"
            }
        }
    }, {
        "nodes": {
            "openvswitchdpdkvhostuserclient": {
                "main": true,
                "Metadata": {
                    "Driver": "openvswitch",
                    "Type": "dpdkvhostuserclient"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportopenvswitchdpdkvhostuserclientlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "openvswitchdpdkvhostuserclient"
            }
        }
    }, {
        "nodes": {
            "net_ixgbedpdk": {
                "main": true,
                "Metadata": {
                    "Driver": "net_ixgbe",
                    "Type": "dpdk"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportnet_ixgbedpdklayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "net_ixgbedpdk"
            }
        }
    }, {
        "nodes": {
            "tuninternal": {
                "main": true,
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
            "ovsbridge": {
                "Metadata": {
                    "Type": "ovsbridge"
                }
            },
        },
        "edges": {
            "ovsporttuninternallayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "tuninternal"
            }, "ovsbridgeovsportownership": {
                "type": "ownership",
                "parent": "ovsbridge",
                "child": "ovsport"
            }, "ovsbridgeovsportlayer2": {
                "type": "layer2",
                "parent": "ovsbridge",
                "child": "ovsport"
            }
        }
    }, {
        "nodes": {
            "vethveth": {
                "main": true,
                "Metadata": {
                    "Driver": "veth",
                    "Type": "veth"
                }
            },
            "ovsport": {
                "Metadata": {
                    "Type": "ovsport"
                }
            },
        },
        "edges": {
            "ovsportvethvethlayer2": {
                "type": "layer2",
                "parent": "ovsport",
                "child": "vethveth"
            }
        }
    }
];
class StructureSimplificationStrategy extends __WEBPACK_IMPORTED_MODULE_1__base__["a" /* default */] {
    findSimilarNodes(skydiveData) {
        let nodes = {};
        let nodeNamesIds = {};
        Object.keys(this.data).forEach((nodeId) => {
            if (!nodeNamesIds[this.data[nodeId].Name]) {
                nodeNamesIds[this.data[nodeId].Name] = new Set();
            }
            nodeNamesIds[this.data[nodeId].Name].add(nodeId);
        });
        Object.keys(nodeNamesIds).forEach((nodeName) => {
            if (nodeNamesIds[nodeName].size == 1) {
                return;
            }
            // try to find a topology from list of hardcoded
            // topologies
            let nodeIdToPatternId = {};
            const patternToBeUsedToSimplifyPartOfGraph = patternsToBeSimplified.find((pattern) => {
                if (nodeNamesIds[nodeName].size !== Object.keys(pattern.nodes).length) {
                    return false;
                }
                let isFoundInPattern = false;
                isFoundInPattern = false;
                nodeIdToPatternId = {};
                for (var it = nodeNamesIds[nodeName].values(), nodeId1 = null; nodeId1 = it.next().value;) {
                    Object.keys(pattern.nodes).forEach((patternID) => {
                        const nodePattern = pattern.nodes[patternID];
                        if (nodePattern.Metadata.Driver && nodePattern.Metadata.Driver !== this.data[nodeId1].Driver) {
                            return;
                        }
                        if (nodePattern.Metadata.Type !== this.data[nodeId1].Type) {
                            return;
                        }
                        nodeIdToPatternId[nodeId1] = (nodePattern.Metadata.Driver || "") + nodePattern.Metadata.Type;
                        isFoundInPattern = true;
                    });
                }
                const edgesToCompare = {};
                skydiveData.Obj.Edges.forEach((edge) => {
                    if (!nodeIdToPatternId[edge.Parent] || !nodeIdToPatternId[edge.Child]) {
                        return;
                    }
                    edgesToCompare[nodeIdToPatternId[edge.Parent] + nodeIdToPatternId[edge.Child] + edge.Metadata.RelationType] = {
                        "type": edge.Metadata.RelationType,
                        "parent": nodeIdToPatternId[edge.Parent],
                        "child": nodeIdToPatternId[edge.Child],
                    };
                });
                if (Object.keys(edgesToCompare).length !== Object.keys(pattern.edges).length) {
                    return false;
                }
                const edges = new Set(Object.keys(edgesToCompare));
                const patternEdges = new Set(Object.keys(pattern.edges));
                let difference = new Set([...edges].filter(x => !patternEdges.has(x)));
                if (difference.size) {
                    return false;
                }
                return true;
            });
            if (!patternToBeUsedToSimplifyPartOfGraph) {
                return;
            }
            const mainNodeId = Object.keys(nodeIdToPatternId).find((nodeId) => {
                return !!patternToBeUsedToSimplifyPartOfGraph.nodes[nodeIdToPatternId[nodeId]].main;
            });
            Object.keys(nodeIdToPatternId).forEach((nodeId) => {
                if (nodeId === mainNodeId) {
                    return;
                }
                nodes[nodeId] = mainNodeId;
            });
        });
        return nodes;
    }
    mergeAllNodes(dataFormat, data, similarNodes) {
        const transformationHandler = __WEBPACK_IMPORTED_MODULE_0__transformation__["a" /* default */].getMergeNodeStrategy(dataFormat);
        return transformationHandler(data, similarNodes);
    }
    simplifyData(dataFormat, data) {
        this.data = Object(__WEBPACK_IMPORTED_MODULE_2__helpers__["b" /* transformSkydiveDataForSimplification */])(data);
        const similarNodes = this.findSimilarNodes(data);
        return this.mergeAllNodes(dataFormat, data, similarNodes);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = StructureSimplificationStrategy;



/***/ }),
/* 32 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_deep_assign__ = __webpack_require__(5);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_deep_assign___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_deep_assign__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__transformation__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__base__ = __webpack_require__(3);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__helpers__ = __webpack_require__(1);




function isEqualHashMap(data, compareTo) {
    return Object.keys(compareTo).every((key, i, arr) => {
        if (typeof compareTo[key] === "string" && compareTo[key] !== data[key]) {
            return false;
        }
        if (typeof compareTo[key] === "string" && compareTo[key] === data[key]) {
            return true;
        }
        return isEqualHashMap(data[key], compareTo[key]);
    });
}
const patternsToBeFlattened = [
    {
        "topology": {
            "first": {
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "nodes": [
                {
                    "node": {
                        "Metadata": {
                            "Driver": "openvswitch",
                            "Type": "patch",
                        }
                    },
                    "edges": [
                        {
                            "Metadata": {
                                "RelationType": "ownership"
                            },
                            "Parent": "parent",
                            "Child": "me",
                        },
                        {
                            "Metadata": {
                                "RelationType": "layer2"
                            },
                            "Parent": "parent",
                            "Child": "me",
                        }
                    ],
                    "nodes": [
                        {
                            "node": {
                                "Metadata": {
                                    "Driver": "openvswitch",
                                    "Type": "patch",
                                }
                            },
                            "edges": [
                                {
                                    "Metadata": {
                                        "RelationType": "layer2",
                                        "Type": "patch"
                                    },
                                    "Parent": "parent",
                                    "Child": "me",
                                }
                            ],
                            "nodes": []
                        }
                    ]
                },
            ]
        }
    },
    {
        "topology": {
            "first": {
                "Metadata": {
                    "Driver": "tun",
                    "Type": "internal"
                }
            },
            "nodes": [
                {
                    "node": {
                        "Metadata": {
                            "Driver": "openvswitch",
                            "Type": "patch",
                        }
                    },
                    "edges": [
                        {
                            "Metadata": {
                                "RelationType": "ownership"
                            },
                            "Parent": "me",
                            "Child": "parent",
                        },
                        {
                            "Metadata": {
                                "RelationType": "layer2"
                            },
                            "Parent": "me",
                            "Child": "parent",
                        }
                    ],
                    "nodes": [
                        {
                            "node": {
                                "Metadata": {
                                    "Driver": "openvswitch",
                                    "Type": "patch",
                                }
                            },
                            "edges": [
                                {
                                    "Metadata": {
                                        "RelationType": "layer2",
                                        "Type": "patch"
                                    },
                                    "Parent": "me",
                                    "Child": "parent",
                                }
                            ],
                            "nodes": []
                        }
                    ]
                },
            ]
        }
    }
];
class FlattenSamplesOfTopologyStrategy extends __WEBPACK_IMPORTED_MODULE_2__base__["a" /* default */] {
    doesConnectionMatchTwoNodes(connection, parentNodeID, childNodeID) {
        if (connection.Metadata.RelationType !== "ownership") {
            if (connection.Child === "me" && !this.data[parentNodeID].layers.has(childNodeID)) {
                return false;
            }
            if (connection.Parent === "parent" && !this.data[childNodeID].layers.has(parentNodeID)) {
                return false;
            }
            if (connection.Child === "parent" && !this.data[childNodeID].layers.has(parentNodeID)) {
                return false;
            }
            if (connection.Parent === "me" && !this.data[parentNodeID].layers.has(childNodeID)) {
                return false;
            }
        }
        if (connection.Metadata.RelationType === "ownership") {
            if (connection.Child === "me" && !this.data[parentNodeID].children.has(childNodeID)) {
                return false;
            }
            if (connection.Parent === "parent" && !this.data[childNodeID].parent.has(parentNodeID)) {
                return false;
            }
        }
        return true;
    }
    returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(mainNode, nodeToCheck) {
        let deadNodes = [];
        const layerConnections = nodeToCheck.edges.reduce((accum, edge) => {
            if (edge.Metadata.RelationType !== "ownership") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        const parentConnections = nodeToCheck.edges.reduce((accum, edge) => {
            if (edge.Metadata.RelationType === "ownership" && edge.Child === "me") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        const childConnections = nodeToCheck.edges.reduce((accum, edge) => {
            if (edge.Metadata.RelationType === "ownership" && edge.Parent === "me") {
                accum.push(nodeToCheck);
            }
            return accum;
        }, []);
        let isMatched = false;
        let matchedNode = null;
        this.data[mainNode.ID].layers.forEach((layerId) => {
            if (isMatched) {
                return;
            }
            layerConnections.forEach((layerData) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({ Metadata: this.data[layerId] }, layerData.node)) {
                    return;
                }
                isMatched = layerData.edges.every((connection) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, layerId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: layerId,
                        Metadata: this.data[layerId]
                    };
                }
            });
        });
        this.data[mainNode.ID].parent.forEach((parentId) => {
            if (isMatched) {
                return;
            }
            parentConnections.forEach((parentData) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({
                    Metadata: this.data[parentId]
                }, parentData.node)) {
                    return;
                }
                isMatched = parentData.edges.every((connection) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, parentId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: parentId,
                        Metadata: this.data[parentId]
                    };
                }
            });
        });
        this.data[mainNode.ID].children.forEach((childrenId) => {
            if (isMatched) {
                return;
            }
            childConnections.forEach((childData) => {
                if (isMatched) {
                    return;
                }
                if (!isEqualHashMap({ Metadata: this.data[childrenId] }, childData.node)) {
                    return;
                }
                isMatched = childData.edges.every((connection) => {
                    return this.doesConnectionMatchTwoNodes(connection, mainNode.ID, childrenId);
                });
                if (isMatched) {
                    matchedNode = {
                        ID: childrenId,
                        Metadata: this.data[childrenId]
                    };
                }
            });
        });
        if (isMatched && nodeToCheck.nodes.length) {
            // console.log('matched node and edges', JSON.stringify(mainNode), JSON.stringify(nodeToCheck));
            nodeToCheck.nodes.every((nodeConnectedTo) => {
                if (!isEqualHashMap(matchedNode, nodeConnectedTo.node)) {
                    return true;
                }
                if (matchedNode.ID === mainNode.ID) {
                    return true;
                }
                const isMatchedAndDeadNodes = this.returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(matchedNode, nodeConnectedTo);
                if (!isMatchedAndDeadNodes.isMatched) {
                    return false;
                }
                deadNodes = deadNodes.concat(isMatchedAndDeadNodes.deadNodes);
                return true;
            });
        }
        else {
            if (nodeToCheck.nodes.length) {
                // console.log('not matched node and edges', JSON.stringify(mainNode), JSON.stringify(nodeToCheck));
            }
            else {
                // console.log('matched but no nodes/edges need to be checked', matchedNode.ID);
                isMatched = true;
            }
        }
        if (isMatched) {
            deadNodes.push(matchedNode.ID);
        }
        return {
            isMatched, deadNodes
        };
    }
    findDeadNodesInSpecificSubTopology(node, topology) {
        let deadNodes = {};
        const isFoundAnyNotMatchedNode = !topology.nodes.every((nodeConnectedTo) => {
            const isMatchedAndDeadNodes = this.returnDeadNodesIfNodesConnectedToNodeMatchExpectedConnections(node, nodeConnectedTo);
            if (!isMatchedAndDeadNodes.isMatched) {
                return false;
            }
            deadNodes = __WEBPACK_IMPORTED_MODULE_0_deep_assign__(deadNodes, isMatchedAndDeadNodes.deadNodes.reduce((accum, deadNode) => {
                accum[deadNode] = node.ID;
                return accum;
            }, {}));
            return true;
        });
        if (isFoundAnyNotMatchedNode) {
            return {};
        }
        return deadNodes;
    }
    flattenSamplesOfTopology(data) {
        let topologyDeadNodes = {};
        const nodesToBeFlattened = {};
        while (true) {
            const notReplaced = patternsToBeFlattened.some((topology) => {
                data.Obj.Nodes.forEach((node) => {
                    if (!isEqualHashMap(node, topology.topology.first)) {
                        return false;
                    }
                    console.log('matched node - try to flat sub graph', node.Metadata.Name, topology.topology.first);
                    const deadNodes = this.findDeadNodesInSpecificSubTopology(node, topology.topology);
                    console.log('matched node - try to flat sub graph. Dead nodes are', deadNodes);
                    if (!Object.keys(deadNodes).length) {
                        return false;
                    }
                    const transformationHandler = __WEBPACK_IMPORTED_MODULE_1__transformation__["a" /* default */].getMergeNodeStrategy("skydive");
                    data = transformationHandler(data, deadNodes);
                    // console.log(JSON.stringify(data));
                    this.data = Object(__WEBPACK_IMPORTED_MODULE_3__helpers__["b" /* transformSkydiveDataForSimplification */])(data);
                    return true;
                });
            });
            if (!notReplaced) {
                break;
            }
        }
        ;
        return data;
    }
    simplifyData(dataFormat, data) {
        this.data = Object(__WEBPACK_IMPORTED_MODULE_3__helpers__["b" /* transformSkydiveDataForSimplification */])(data);
        data = this.flattenSamplesOfTopology(data);
        return data;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = FlattenSamplesOfTopologyStrategy;



/***/ }),
/* 33 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
const get = __webpack_require__(34);
const set = __webpack_require__(36);
class LayoutConfig {
    constructor(configuration) {
        this.configuration = configuration;
    }
    getValue(pathInConfig, ...args) {
        const val = get(this.configuration, pathInConfig);
        if (typeof val === 'function') {
            return val(...args);
        }
        return val;
    }
    setValue(pathInConfig, val) {
        set(this.configuration, pathInConfig, val);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutConfig;



/***/ }),
/* 34 */
/***/ (function(module, exports, __webpack_require__) {

/*!
 * get-value <https://github.com/jonschlinkert/get-value>
 *
 * Copyright (c) 2014-2018, Jon Schlinkert.
 * Released under the MIT License.
 */

const isObject = __webpack_require__(6);

module.exports = function(target, path, options) {
  if (!isObject(options)) {
    options = { default: options };
  }

  if (!isValidObject(target)) {
    return typeof options.default !== 'undefined' ? options.default : target;
  }

  if (typeof path === 'number') {
    path = String(path);
  }

  const isArray = Array.isArray(path);
  const isString = typeof path === 'string';
  const splitChar = options.separator || '.';
  const joinChar = options.joinChar || (typeof splitChar === 'string' ? splitChar : '.');

  if (!isString && !isArray) {
    return target;
  }

  if (isString && path in target) {
    return isValid(path, target, options) ? target[path] : options.default;
  }

  let segs = isArray ? path : split(path, splitChar, options);
  let len = segs.length;
  let idx = 0;

  do {
    let prop = segs[idx];
    if (typeof prop === 'number') {
      prop = String(prop);
    }

    while (prop && prop.slice(-1) === '\\') {
      prop = join([prop.slice(0, -1), segs[++idx] || ''], joinChar, options);
    }

    if (prop in target) {
      if (!isValid(prop, target, options)) {
        return options.default;
      }

      target = target[prop];
    } else {
      let hasProp = false;
      let n = idx + 1;

      while (n < len) {
        prop = join([prop, segs[n++]], joinChar, options);

        if ((hasProp = prop in target)) {
          if (!isValid(prop, target, options)) {
            return options.default;
          }

          target = target[prop];
          idx = n - 1;
          break;
        }
      }

      if (!hasProp) {
        return options.default;
      }
    }
  } while (++idx < len && isValidObject(target));

  if (idx === len) {
    return target;
  }

  return options.default;
};

function join(segs, joinChar, options) {
  if (typeof options.join === 'function') {
    return options.join(segs);
  }
  return segs[0] + joinChar + segs[1];
}

function split(path, splitChar, options) {
  if (typeof options.split === 'function') {
    return options.split(path);
  }
  return path.split(splitChar);
}

function isValid(key, target, options) {
  if (typeof options.isValid === 'function') {
    return options.isValid(key, target);
  }
  return true;
}

function isValidObject(val) {
  return isObject(val) || Array.isArray(val) || typeof val === 'function';
}


/***/ }),
/* 35 */
/***/ (function(module, exports) {

var toString = {}.toString;

module.exports = Array.isArray || function (arr) {
  return toString.call(arr) == '[object Array]';
};


/***/ }),
/* 36 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * set-value <https://github.com/jonschlinkert/set-value>
 *
 * Copyright (c) 2014-2018, Jon Schlinkert.
 * Released under the MIT License.
 */



const isPlain = __webpack_require__(37);

function set(target, path, value, options) {
  if (!isObject(target)) {
    return target;
  }

  let opts = options || {};
  const isArray = Array.isArray(path);
  if (!isArray && typeof path !== 'string') {
    return target;
  }

  let merge = opts.merge;
  if (merge && typeof merge !== 'function') {
    merge = Object.assign;
  }

  const keys = isArray ? path : split(path, opts);
  const len = keys.length;
  const orig = target;

  if (!options && keys.length === 1) {
    result(target, keys[0], value, merge);
    return target;
  }

  for (let i = 0; i < len; i++) {
    let prop = keys[i];

    if (!isObject(target[prop])) {
      target[prop] = {};
    }

    if (i === len - 1) {
      result(target, prop, value, merge);
      break;
    }

    target = target[prop];
  }

  return orig;
}

function result(target, path, value, merge) {
  if (merge && isPlain(target[path]) && isPlain(value)) {
    target[path] = merge({}, target[path], value);
  } else {
    target[path] = value;
  }
}

function split(path, options) {
  const id = createKey(path, options);
  if (set.memo[id]) return set.memo[id];

  const char = (options && options.separator) ? options.separator : '.';
  let keys = [];
  let res = [];

  if (options && typeof options.split === 'function') {
    keys = options.split(path);
  } else {
    keys = path.split(char);
  }

  for (let i = 0; i < keys.length; i++) {
    let prop = keys[i];
    while (prop && prop.slice(-1) === '\\' && keys[i + 1]) {
      prop = prop.slice(0, -1) + char + keys[++i];
    }
    res.push(prop);
  }
  set.memo[id] = res;
  return res;
}

function createKey(pattern, options) {
  let id = pattern;
  if (typeof options === 'undefined') {
    return id + '';
  }
  const keys = Object.keys(options);
  for (let i = 0; i < keys.length; i++) {
    const key = keys[i];
    id += ';' + key + '=' + String(options[key]);
  }
  return id;
}

function isObject(val) {
  switch (typeof val) {
    case 'null':
      return false;
    case 'object':
      return true;
    case 'function':
      return true;
    default: {
      return false;
    }
  }
}

set.memo = {};
module.exports = set;


/***/ }),
/* 37 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * is-plain-object <https://github.com/jonschlinkert/is-plain-object>
 *
 * Copyright (c) 2014-2017, Jon Schlinkert.
 * Released under the MIT License.
 */



var isObject = __webpack_require__(6);

function isObjectObject(o) {
  return isObject(o) === true
    && Object.prototype.toString.call(o) === '[object Object]';
}

module.exports = function isPlainObject(o) {
  var ctor,prot;

  if (isObjectObject(o) === false) return false;

  // If has modified constructor
  ctor = o.constructor;
  if (typeof ctor !== 'function') return false;

  // If has modified prototype
  prot = ctor.prototype;
  if (isObjectObject(prot) === false) return false;

  // If constructor does not have an Object-specific method
  if (prot.hasOwnProperty('isPrototypeOf') === false) {
    return false;
  }

  // Most likely a plain Object
  return true;
};


/***/ }),
/* 38 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__topology_layout_registry__ = __webpack_require__(39);

class TopologyManager {
    constructor() {
        this.layouts = new __WEBPACK_IMPORTED_MODULE_0__topology_layout_registry__["a" /* default */]();
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = TopologyManager;



/***/ }),
/* 39 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class LayoutRegistry {
    constructor() {
        this.layouts = [];
    }
    add(layout) {
        this.layouts.push(layout);
    }
    activate(layoutAlias) {
        this.layouts.forEach(l => {
            if (l.alias !== layoutAlias) {
                if (l.active) {
                    l.remove();
                }
                return;
            }
            l.initializer();
        });
    }
    getActive() {
        return this.layouts.find(l => {
            if (l.active) {
                return true;
            }
            return false;
        });
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutRegistry;



/***/ }),
/* 40 */
/***/ (function(module, exports, __webpack_require__) {

// https://d3js.org/d3-hierarchy/ v1.1.8 Copyright 2018 Mike Bostock
(function (global, factory) {
 true ? factory(exports) :
typeof define === 'function' && define.amd ? define(['exports'], factory) :
(factory((global.d3 = global.d3 || {})));
}(this, (function (exports) { 'use strict';

function defaultSeparation(a, b) {
  return a.parent === b.parent ? 1 : 2;
}

function meanX(children) {
  return children.reduce(meanXReduce, 0) / children.length;
}

function meanXReduce(x, c) {
  return x + c.x;
}

function maxY(children) {
  return 1 + children.reduce(maxYReduce, 0);
}

function maxYReduce(y, c) {
  return Math.max(y, c.y);
}

function leafLeft(node) {
  var children;
  while (children = node.children) node = children[0];
  return node;
}

function leafRight(node) {
  var children;
  while (children = node.children) node = children[children.length - 1];
  return node;
}

function cluster() {
  var separation = defaultSeparation,
      dx = 1,
      dy = 1,
      nodeSize = false;

  function cluster(root) {
    var previousNode,
        x = 0;

    // First walk, computing the initial x & y values.
    root.eachAfter(function(node) {
      var children = node.children;
      if (children) {
        node.x = meanX(children);
        node.y = maxY(children);
      } else {
        node.x = previousNode ? x += separation(node, previousNode) : 0;
        node.y = 0;
        previousNode = node;
      }
    });

    var left = leafLeft(root),
        right = leafRight(root),
        x0 = left.x - separation(left, right) / 2,
        x1 = right.x + separation(right, left) / 2;

    // Second walk, normalizing x & y to the desired size.
    return root.eachAfter(nodeSize ? function(node) {
      node.x = (node.x - root.x) * dx;
      node.y = (root.y - node.y) * dy;
    } : function(node) {
      node.x = (node.x - x0) / (x1 - x0) * dx;
      node.y = (1 - (root.y ? node.y / root.y : 1)) * dy;
    });
  }

  cluster.separation = function(x) {
    return arguments.length ? (separation = x, cluster) : separation;
  };

  cluster.size = function(x) {
    return arguments.length ? (nodeSize = false, dx = +x[0], dy = +x[1], cluster) : (nodeSize ? null : [dx, dy]);
  };

  cluster.nodeSize = function(x) {
    return arguments.length ? (nodeSize = true, dx = +x[0], dy = +x[1], cluster) : (nodeSize ? [dx, dy] : null);
  };

  return cluster;
}

function count(node) {
  var sum = 0,
      children = node.children,
      i = children && children.length;
  if (!i) sum = 1;
  else while (--i >= 0) sum += children[i].value;
  node.value = sum;
}

function node_count() {
  return this.eachAfter(count);
}

function node_each(callback) {
  var node = this, current, next = [node], children, i, n;
  do {
    current = next.reverse(), next = [];
    while (node = current.pop()) {
      callback(node), children = node.children;
      if (children) for (i = 0, n = children.length; i < n; ++i) {
        next.push(children[i]);
      }
    }
  } while (next.length);
  return this;
}

function node_eachBefore(callback) {
  var node = this, nodes = [node], children, i;
  while (node = nodes.pop()) {
    callback(node), children = node.children;
    if (children) for (i = children.length - 1; i >= 0; --i) {
      nodes.push(children[i]);
    }
  }
  return this;
}

function node_eachAfter(callback) {
  var node = this, nodes = [node], next = [], children, i, n;
  while (node = nodes.pop()) {
    next.push(node), children = node.children;
    if (children) for (i = 0, n = children.length; i < n; ++i) {
      nodes.push(children[i]);
    }
  }
  while (node = next.pop()) {
    callback(node);
  }
  return this;
}

function node_sum(value) {
  return this.eachAfter(function(node) {
    var sum = +value(node.data) || 0,
        children = node.children,
        i = children && children.length;
    while (--i >= 0) sum += children[i].value;
    node.value = sum;
  });
}

function node_sort(compare) {
  return this.eachBefore(function(node) {
    if (node.children) {
      node.children.sort(compare);
    }
  });
}

function node_path(end) {
  var start = this,
      ancestor = leastCommonAncestor(start, end),
      nodes = [start];
  while (start !== ancestor) {
    start = start.parent;
    nodes.push(start);
  }
  var k = nodes.length;
  while (end !== ancestor) {
    nodes.splice(k, 0, end);
    end = end.parent;
  }
  return nodes;
}

function leastCommonAncestor(a, b) {
  if (a === b) return a;
  var aNodes = a.ancestors(),
      bNodes = b.ancestors(),
      c = null;
  a = aNodes.pop();
  b = bNodes.pop();
  while (a === b) {
    c = a;
    a = aNodes.pop();
    b = bNodes.pop();
  }
  return c;
}

function node_ancestors() {
  var node = this, nodes = [node];
  while (node = node.parent) {
    nodes.push(node);
  }
  return nodes;
}

function node_descendants() {
  var nodes = [];
  this.each(function(node) {
    nodes.push(node);
  });
  return nodes;
}

function node_leaves() {
  var leaves = [];
  this.eachBefore(function(node) {
    if (!node.children) {
      leaves.push(node);
    }
  });
  return leaves;
}

function node_links() {
  var root = this, links = [];
  root.each(function(node) {
    if (node !== root) { // Don’t include the root’s parent, if any.
      links.push({source: node.parent, target: node});
    }
  });
  return links;
}

function hierarchy(data, children) {
  var root = new Node(data),
      valued = +data.value && (root.value = data.value),
      node,
      nodes = [root],
      child,
      childs,
      i,
      n;

  if (children == null) children = defaultChildren;

  while (node = nodes.pop()) {
    if (valued) node.value = +node.data.value;
    if ((childs = children(node.data)) && (n = childs.length)) {
      node.children = new Array(n);
      for (i = n - 1; i >= 0; --i) {
        nodes.push(child = node.children[i] = new Node(childs[i]));
        child.parent = node;
        child.depth = node.depth + 1;
      }
    }
  }

  return root.eachBefore(computeHeight);
}

function node_copy() {
  return hierarchy(this).eachBefore(copyData);
}

function defaultChildren(d) {
  return d.children;
}

function copyData(node) {
  node.data = node.data.data;
}

function computeHeight(node) {
  var height = 0;
  do node.height = height;
  while ((node = node.parent) && (node.height < ++height));
}

function Node(data) {
  this.data = data;
  this.depth =
  this.height = 0;
  this.parent = null;
}

Node.prototype = hierarchy.prototype = {
  constructor: Node,
  count: node_count,
  each: node_each,
  eachAfter: node_eachAfter,
  eachBefore: node_eachBefore,
  sum: node_sum,
  sort: node_sort,
  path: node_path,
  ancestors: node_ancestors,
  descendants: node_descendants,
  leaves: node_leaves,
  links: node_links,
  copy: node_copy
};

var slice = Array.prototype.slice;

function shuffle(array) {
  var m = array.length,
      t,
      i;

  while (m) {
    i = Math.random() * m-- | 0;
    t = array[m];
    array[m] = array[i];
    array[i] = t;
  }

  return array;
}

function enclose(circles) {
  var i = 0, n = (circles = shuffle(slice.call(circles))).length, B = [], p, e;

  while (i < n) {
    p = circles[i];
    if (e && enclosesWeak(e, p)) ++i;
    else e = encloseBasis(B = extendBasis(B, p)), i = 0;
  }

  return e;
}

function extendBasis(B, p) {
  var i, j;

  if (enclosesWeakAll(p, B)) return [p];

  // If we get here then B must have at least one element.
  for (i = 0; i < B.length; ++i) {
    if (enclosesNot(p, B[i])
        && enclosesWeakAll(encloseBasis2(B[i], p), B)) {
      return [B[i], p];
    }
  }

  // If we get here then B must have at least two elements.
  for (i = 0; i < B.length - 1; ++i) {
    for (j = i + 1; j < B.length; ++j) {
      if (enclosesNot(encloseBasis2(B[i], B[j]), p)
          && enclosesNot(encloseBasis2(B[i], p), B[j])
          && enclosesNot(encloseBasis2(B[j], p), B[i])
          && enclosesWeakAll(encloseBasis3(B[i], B[j], p), B)) {
        return [B[i], B[j], p];
      }
    }
  }

  // If we get here then something is very wrong.
  throw new Error;
}

function enclosesNot(a, b) {
  var dr = a.r - b.r, dx = b.x - a.x, dy = b.y - a.y;
  return dr < 0 || dr * dr < dx * dx + dy * dy;
}

function enclosesWeak(a, b) {
  var dr = a.r - b.r + 1e-6, dx = b.x - a.x, dy = b.y - a.y;
  return dr > 0 && dr * dr > dx * dx + dy * dy;
}

function enclosesWeakAll(a, B) {
  for (var i = 0; i < B.length; ++i) {
    if (!enclosesWeak(a, B[i])) {
      return false;
    }
  }
  return true;
}

function encloseBasis(B) {
  switch (B.length) {
    case 1: return encloseBasis1(B[0]);
    case 2: return encloseBasis2(B[0], B[1]);
    case 3: return encloseBasis3(B[0], B[1], B[2]);
  }
}

function encloseBasis1(a) {
  return {
    x: a.x,
    y: a.y,
    r: a.r
  };
}

function encloseBasis2(a, b) {
  var x1 = a.x, y1 = a.y, r1 = a.r,
      x2 = b.x, y2 = b.y, r2 = b.r,
      x21 = x2 - x1, y21 = y2 - y1, r21 = r2 - r1,
      l = Math.sqrt(x21 * x21 + y21 * y21);
  return {
    x: (x1 + x2 + x21 / l * r21) / 2,
    y: (y1 + y2 + y21 / l * r21) / 2,
    r: (l + r1 + r2) / 2
  };
}

function encloseBasis3(a, b, c) {
  var x1 = a.x, y1 = a.y, r1 = a.r,
      x2 = b.x, y2 = b.y, r2 = b.r,
      x3 = c.x, y3 = c.y, r3 = c.r,
      a2 = x1 - x2,
      a3 = x1 - x3,
      b2 = y1 - y2,
      b3 = y1 - y3,
      c2 = r2 - r1,
      c3 = r3 - r1,
      d1 = x1 * x1 + y1 * y1 - r1 * r1,
      d2 = d1 - x2 * x2 - y2 * y2 + r2 * r2,
      d3 = d1 - x3 * x3 - y3 * y3 + r3 * r3,
      ab = a3 * b2 - a2 * b3,
      xa = (b2 * d3 - b3 * d2) / (ab * 2) - x1,
      xb = (b3 * c2 - b2 * c3) / ab,
      ya = (a3 * d2 - a2 * d3) / (ab * 2) - y1,
      yb = (a2 * c3 - a3 * c2) / ab,
      A = xb * xb + yb * yb - 1,
      B = 2 * (r1 + xa * xb + ya * yb),
      C = xa * xa + ya * ya - r1 * r1,
      r = -(A ? (B + Math.sqrt(B * B - 4 * A * C)) / (2 * A) : C / B);
  return {
    x: x1 + xa + xb * r,
    y: y1 + ya + yb * r,
    r: r
  };
}

function place(b, a, c) {
  var dx = b.x - a.x, x, a2,
      dy = b.y - a.y, y, b2,
      d2 = dx * dx + dy * dy;
  if (d2) {
    a2 = a.r + c.r, a2 *= a2;
    b2 = b.r + c.r, b2 *= b2;
    if (a2 > b2) {
      x = (d2 + b2 - a2) / (2 * d2);
      y = Math.sqrt(Math.max(0, b2 / d2 - x * x));
      c.x = b.x - x * dx - y * dy;
      c.y = b.y - x * dy + y * dx;
    } else {
      x = (d2 + a2 - b2) / (2 * d2);
      y = Math.sqrt(Math.max(0, a2 / d2 - x * x));
      c.x = a.x + x * dx - y * dy;
      c.y = a.y + x * dy + y * dx;
    }
  } else {
    c.x = a.x + c.r;
    c.y = a.y;
  }
}

function intersects(a, b) {
  var dr = a.r + b.r - 1e-6, dx = b.x - a.x, dy = b.y - a.y;
  return dr > 0 && dr * dr > dx * dx + dy * dy;
}

function score(node) {
  var a = node._,
      b = node.next._,
      ab = a.r + b.r,
      dx = (a.x * b.r + b.x * a.r) / ab,
      dy = (a.y * b.r + b.y * a.r) / ab;
  return dx * dx + dy * dy;
}

function Node$1(circle) {
  this._ = circle;
  this.next = null;
  this.previous = null;
}

function packEnclose(circles) {
  if (!(n = circles.length)) return 0;

  var a, b, c, n, aa, ca, i, j, k, sj, sk;

  // Place the first circle.
  a = circles[0], a.x = 0, a.y = 0;
  if (!(n > 1)) return a.r;

  // Place the second circle.
  b = circles[1], a.x = -b.r, b.x = a.r, b.y = 0;
  if (!(n > 2)) return a.r + b.r;

  // Place the third circle.
  place(b, a, c = circles[2]);

  // Initialize the front-chain using the first three circles a, b and c.
  a = new Node$1(a), b = new Node$1(b), c = new Node$1(c);
  a.next = c.previous = b;
  b.next = a.previous = c;
  c.next = b.previous = a;

  // Attempt to place each remaining circle…
  pack: for (i = 3; i < n; ++i) {
    place(a._, b._, c = circles[i]), c = new Node$1(c);

    // Find the closest intersecting circle on the front-chain, if any.
    // “Closeness” is determined by linear distance along the front-chain.
    // “Ahead” or “behind” is likewise determined by linear distance.
    j = b.next, k = a.previous, sj = b._.r, sk = a._.r;
    do {
      if (sj <= sk) {
        if (intersects(j._, c._)) {
          b = j, a.next = b, b.previous = a, --i;
          continue pack;
        }
        sj += j._.r, j = j.next;
      } else {
        if (intersects(k._, c._)) {
          a = k, a.next = b, b.previous = a, --i;
          continue pack;
        }
        sk += k._.r, k = k.previous;
      }
    } while (j !== k.next);

    // Success! Insert the new circle c between a and b.
    c.previous = a, c.next = b, a.next = b.previous = b = c;

    // Compute the new closest circle pair to the centroid.
    aa = score(a);
    while ((c = c.next) !== b) {
      if ((ca = score(c)) < aa) {
        a = c, aa = ca;
      }
    }
    b = a.next;
  }

  // Compute the enclosing circle of the front chain.
  a = [b._], c = b; while ((c = c.next) !== b) a.push(c._); c = enclose(a);

  // Translate the circles to put the enclosing circle around the origin.
  for (i = 0; i < n; ++i) a = circles[i], a.x -= c.x, a.y -= c.y;

  return c.r;
}

function siblings(circles) {
  packEnclose(circles);
  return circles;
}

function optional(f) {
  return f == null ? null : required(f);
}

function required(f) {
  if (typeof f !== "function") throw new Error;
  return f;
}

function constantZero() {
  return 0;
}

function constant(x) {
  return function() {
    return x;
  };
}

function defaultRadius(d) {
  return Math.sqrt(d.value);
}

function index() {
  var radius = null,
      dx = 1,
      dy = 1,
      padding = constantZero;

  function pack(root) {
    root.x = dx / 2, root.y = dy / 2;
    if (radius) {
      root.eachBefore(radiusLeaf(radius))
          .eachAfter(packChildren(padding, 0.5))
          .eachBefore(translateChild(1));
    } else {
      root.eachBefore(radiusLeaf(defaultRadius))
          .eachAfter(packChildren(constantZero, 1))
          .eachAfter(packChildren(padding, root.r / Math.min(dx, dy)))
          .eachBefore(translateChild(Math.min(dx, dy) / (2 * root.r)));
    }
    return root;
  }

  pack.radius = function(x) {
    return arguments.length ? (radius = optional(x), pack) : radius;
  };

  pack.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], pack) : [dx, dy];
  };

  pack.padding = function(x) {
    return arguments.length ? (padding = typeof x === "function" ? x : constant(+x), pack) : padding;
  };

  return pack;
}

function radiusLeaf(radius) {
  return function(node) {
    if (!node.children) {
      node.r = Math.max(0, +radius(node) || 0);
    }
  };
}

function packChildren(padding, k) {
  return function(node) {
    if (children = node.children) {
      var children,
          i,
          n = children.length,
          r = padding(node) * k || 0,
          e;

      if (r) for (i = 0; i < n; ++i) children[i].r += r;
      e = packEnclose(children);
      if (r) for (i = 0; i < n; ++i) children[i].r -= r;
      node.r = e + r;
    }
  };
}

function translateChild(k) {
  return function(node) {
    var parent = node.parent;
    node.r *= k;
    if (parent) {
      node.x = parent.x + k * node.x;
      node.y = parent.y + k * node.y;
    }
  };
}

function roundNode(node) {
  node.x0 = Math.round(node.x0);
  node.y0 = Math.round(node.y0);
  node.x1 = Math.round(node.x1);
  node.y1 = Math.round(node.y1);
}

function treemapDice(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      node,
      i = -1,
      n = nodes.length,
      k = parent.value && (x1 - x0) / parent.value;

  while (++i < n) {
    node = nodes[i], node.y0 = y0, node.y1 = y1;
    node.x0 = x0, node.x1 = x0 += node.value * k;
  }
}

function partition() {
  var dx = 1,
      dy = 1,
      padding = 0,
      round = false;

  function partition(root) {
    var n = root.height + 1;
    root.x0 =
    root.y0 = padding;
    root.x1 = dx;
    root.y1 = dy / n;
    root.eachBefore(positionNode(dy, n));
    if (round) root.eachBefore(roundNode);
    return root;
  }

  function positionNode(dy, n) {
    return function(node) {
      if (node.children) {
        treemapDice(node, node.x0, dy * (node.depth + 1) / n, node.x1, dy * (node.depth + 2) / n);
      }
      var x0 = node.x0,
          y0 = node.y0,
          x1 = node.x1 - padding,
          y1 = node.y1 - padding;
      if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
      if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
      node.x0 = x0;
      node.y0 = y0;
      node.x1 = x1;
      node.y1 = y1;
    };
  }

  partition.round = function(x) {
    return arguments.length ? (round = !!x, partition) : round;
  };

  partition.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], partition) : [dx, dy];
  };

  partition.padding = function(x) {
    return arguments.length ? (padding = +x, partition) : padding;
  };

  return partition;
}

var keyPrefix = "$", // Protect against keys like “__proto__”.
    preroot = {depth: -1},
    ambiguous = {};

function defaultId(d) {
  return d.id;
}

function defaultParentId(d) {
  return d.parentId;
}

function stratify() {
  var id = defaultId,
      parentId = defaultParentId;

  function stratify(data) {
    var d,
        i,
        n = data.length,
        root,
        parent,
        node,
        nodes = new Array(n),
        nodeId,
        nodeKey,
        nodeByKey = {};

    for (i = 0; i < n; ++i) {
      d = data[i], node = nodes[i] = new Node(d);
      if ((nodeId = id(d, i, data)) != null && (nodeId += "")) {
        nodeKey = keyPrefix + (node.id = nodeId);
        nodeByKey[nodeKey] = nodeKey in nodeByKey ? ambiguous : node;
      }
    }

    for (i = 0; i < n; ++i) {
      node = nodes[i], nodeId = parentId(data[i], i, data);
      if (nodeId == null || !(nodeId += "")) {
        if (root) throw new Error("multiple roots");
        root = node;
      } else {
        parent = nodeByKey[keyPrefix + nodeId];
        if (!parent) throw new Error("missing: " + nodeId);
        if (parent === ambiguous) throw new Error("ambiguous: " + nodeId);
        if (parent.children) parent.children.push(node);
        else parent.children = [node];
        node.parent = parent;
      }
    }

    if (!root) throw new Error("no root");
    root.parent = preroot;
    root.eachBefore(function(node) { node.depth = node.parent.depth + 1; --n; }).eachBefore(computeHeight);
    root.parent = null;
    if (n > 0) throw new Error("cycle");

    return root;
  }

  stratify.id = function(x) {
    return arguments.length ? (id = required(x), stratify) : id;
  };

  stratify.parentId = function(x) {
    return arguments.length ? (parentId = required(x), stratify) : parentId;
  };

  return stratify;
}

function defaultSeparation$1(a, b) {
  return a.parent === b.parent ? 1 : 2;
}

// function radialSeparation(a, b) {
//   return (a.parent === b.parent ? 1 : 2) / a.depth;
// }

// This function is used to traverse the left contour of a subtree (or
// subforest). It returns the successor of v on this contour. This successor is
// either given by the leftmost child of v or by the thread of v. The function
// returns null if and only if v is on the highest level of its subtree.
function nextLeft(v) {
  var children = v.children;
  return children ? children[0] : v.t;
}

// This function works analogously to nextLeft.
function nextRight(v) {
  var children = v.children;
  return children ? children[children.length - 1] : v.t;
}

// Shifts the current subtree rooted at w+. This is done by increasing
// prelim(w+) and mod(w+) by shift.
function moveSubtree(wm, wp, shift) {
  var change = shift / (wp.i - wm.i);
  wp.c -= change;
  wp.s += shift;
  wm.c += change;
  wp.z += shift;
  wp.m += shift;
}

// All other shifts, applied to the smaller subtrees between w- and w+, are
// performed by this function. To prepare the shifts, we have to adjust
// change(w+), shift(w+), and change(w-).
function executeShifts(v) {
  var shift = 0,
      change = 0,
      children = v.children,
      i = children.length,
      w;
  while (--i >= 0) {
    w = children[i];
    w.z += shift;
    w.m += shift;
    shift += w.s + (change += w.c);
  }
}

// If vi-’s ancestor is a sibling of v, returns vi-’s ancestor. Otherwise,
// returns the specified (default) ancestor.
function nextAncestor(vim, v, ancestor) {
  return vim.a.parent === v.parent ? vim.a : ancestor;
}

function TreeNode(node, i) {
  this._ = node;
  this.parent = null;
  this.children = null;
  this.A = null; // default ancestor
  this.a = this; // ancestor
  this.z = 0; // prelim
  this.m = 0; // mod
  this.c = 0; // change
  this.s = 0; // shift
  this.t = null; // thread
  this.i = i; // number
}

TreeNode.prototype = Object.create(Node.prototype);

function treeRoot(root) {
  var tree = new TreeNode(root, 0),
      node,
      nodes = [tree],
      child,
      children,
      i,
      n;

  while (node = nodes.pop()) {
    if (children = node._.children) {
      node.children = new Array(n = children.length);
      for (i = n - 1; i >= 0; --i) {
        nodes.push(child = node.children[i] = new TreeNode(children[i], i));
        child.parent = node;
      }
    }
  }

  (tree.parent = new TreeNode(null, 0)).children = [tree];
  return tree;
}

// Node-link tree diagram using the Reingold-Tilford "tidy" algorithm
function tree() {
  var separation = defaultSeparation$1,
      dx = 1,
      dy = 1,
      nodeSize = null;

  function tree(root) {
    var t = treeRoot(root);

    // Compute the layout using Buchheim et al.’s algorithm.
    t.eachAfter(firstWalk), t.parent.m = -t.z;
    t.eachBefore(secondWalk);

    // If a fixed node size is specified, scale x and y.
    if (nodeSize) root.eachBefore(sizeNode);

    // If a fixed tree size is specified, scale x and y based on the extent.
    // Compute the left-most, right-most, and depth-most nodes for extents.
    else {
      var left = root,
          right = root,
          bottom = root;
      root.eachBefore(function(node) {
        if (node.x < left.x) left = node;
        if (node.x > right.x) right = node;
        if (node.depth > bottom.depth) bottom = node;
      });
      var s = left === right ? 1 : separation(left, right) / 2,
          tx = s - left.x,
          kx = dx / (right.x + s + tx),
          ky = dy / (bottom.depth || 1);
      root.eachBefore(function(node) {
        node.x = (node.x + tx) * kx;
        node.y = node.depth * ky;
      });
    }

    return root;
  }

  // Computes a preliminary x-coordinate for v. Before that, FIRST WALK is
  // applied recursively to the children of v, as well as the function
  // APPORTION. After spacing out the children by calling EXECUTE SHIFTS, the
  // node v is placed to the midpoint of its outermost children.
  function firstWalk(v) {
    var children = v.children,
        siblings = v.parent.children,
        w = v.i ? siblings[v.i - 1] : null;
    if (children) {
      executeShifts(v);
      var midpoint = (children[0].z + children[children.length - 1].z) / 2;
      if (w) {
        v.z = w.z + separation(v._, w._);
        v.m = v.z - midpoint;
      } else {
        v.z = midpoint;
      }
    } else if (w) {
      v.z = w.z + separation(v._, w._);
    }
    v.parent.A = apportion(v, w, v.parent.A || siblings[0]);
  }

  // Computes all real x-coordinates by summing up the modifiers recursively.
  function secondWalk(v) {
    v._.x = v.z + v.parent.m;
    v.m += v.parent.m;
  }

  // The core of the algorithm. Here, a new subtree is combined with the
  // previous subtrees. Threads are used to traverse the inside and outside
  // contours of the left and right subtree up to the highest common level. The
  // vertices used for the traversals are vi+, vi-, vo-, and vo+, where the
  // superscript o means outside and i means inside, the subscript - means left
  // subtree and + means right subtree. For summing up the modifiers along the
  // contour, we use respective variables si+, si-, so-, and so+. Whenever two
  // nodes of the inside contours conflict, we compute the left one of the
  // greatest uncommon ancestors using the function ANCESTOR and call MOVE
  // SUBTREE to shift the subtree and prepare the shifts of smaller subtrees.
  // Finally, we add a new thread (if necessary).
  function apportion(v, w, ancestor) {
    if (w) {
      var vip = v,
          vop = v,
          vim = w,
          vom = vip.parent.children[0],
          sip = vip.m,
          sop = vop.m,
          sim = vim.m,
          som = vom.m,
          shift;
      while (vim = nextRight(vim), vip = nextLeft(vip), vim && vip) {
        vom = nextLeft(vom);
        vop = nextRight(vop);
        vop.a = v;
        shift = vim.z + sim - vip.z - sip + separation(vim._, vip._);
        if (shift > 0) {
          moveSubtree(nextAncestor(vim, v, ancestor), v, shift);
          sip += shift;
          sop += shift;
        }
        sim += vim.m;
        sip += vip.m;
        som += vom.m;
        sop += vop.m;
      }
      if (vim && !nextRight(vop)) {
        vop.t = vim;
        vop.m += sim - sop;
      }
      if (vip && !nextLeft(vom)) {
        vom.t = vip;
        vom.m += sip - som;
        ancestor = v;
      }
    }
    return ancestor;
  }

  function sizeNode(node) {
    node.x *= dx;
    node.y = node.depth * dy;
  }

  tree.separation = function(x) {
    return arguments.length ? (separation = x, tree) : separation;
  };

  tree.size = function(x) {
    return arguments.length ? (nodeSize = false, dx = +x[0], dy = +x[1], tree) : (nodeSize ? null : [dx, dy]);
  };

  tree.nodeSize = function(x) {
    return arguments.length ? (nodeSize = true, dx = +x[0], dy = +x[1], tree) : (nodeSize ? [dx, dy] : null);
  };

  return tree;
}

function treemapSlice(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      node,
      i = -1,
      n = nodes.length,
      k = parent.value && (y1 - y0) / parent.value;

  while (++i < n) {
    node = nodes[i], node.x0 = x0, node.x1 = x1;
    node.y0 = y0, node.y1 = y0 += node.value * k;
  }
}

var phi = (1 + Math.sqrt(5)) / 2;

function squarifyRatio(ratio, parent, x0, y0, x1, y1) {
  var rows = [],
      nodes = parent.children,
      row,
      nodeValue,
      i0 = 0,
      i1 = 0,
      n = nodes.length,
      dx, dy,
      value = parent.value,
      sumValue,
      minValue,
      maxValue,
      newRatio,
      minRatio,
      alpha,
      beta;

  while (i0 < n) {
    dx = x1 - x0, dy = y1 - y0;

    // Find the next non-empty node.
    do sumValue = nodes[i1++].value; while (!sumValue && i1 < n);
    minValue = maxValue = sumValue;
    alpha = Math.max(dy / dx, dx / dy) / (value * ratio);
    beta = sumValue * sumValue * alpha;
    minRatio = Math.max(maxValue / beta, beta / minValue);

    // Keep adding nodes while the aspect ratio maintains or improves.
    for (; i1 < n; ++i1) {
      sumValue += nodeValue = nodes[i1].value;
      if (nodeValue < minValue) minValue = nodeValue;
      if (nodeValue > maxValue) maxValue = nodeValue;
      beta = sumValue * sumValue * alpha;
      newRatio = Math.max(maxValue / beta, beta / minValue);
      if (newRatio > minRatio) { sumValue -= nodeValue; break; }
      minRatio = newRatio;
    }

    // Position and record the row orientation.
    rows.push(row = {value: sumValue, dice: dx < dy, children: nodes.slice(i0, i1)});
    if (row.dice) treemapDice(row, x0, y0, x1, value ? y0 += dy * sumValue / value : y1);
    else treemapSlice(row, x0, y0, value ? x0 += dx * sumValue / value : x1, y1);
    value -= sumValue, i0 = i1;
  }

  return rows;
}

var squarify = (function custom(ratio) {

  function squarify(parent, x0, y0, x1, y1) {
    squarifyRatio(ratio, parent, x0, y0, x1, y1);
  }

  squarify.ratio = function(x) {
    return custom((x = +x) > 1 ? x : 1);
  };

  return squarify;
})(phi);

function index$1() {
  var tile = squarify,
      round = false,
      dx = 1,
      dy = 1,
      paddingStack = [0],
      paddingInner = constantZero,
      paddingTop = constantZero,
      paddingRight = constantZero,
      paddingBottom = constantZero,
      paddingLeft = constantZero;

  function treemap(root) {
    root.x0 =
    root.y0 = 0;
    root.x1 = dx;
    root.y1 = dy;
    root.eachBefore(positionNode);
    paddingStack = [0];
    if (round) root.eachBefore(roundNode);
    return root;
  }

  function positionNode(node) {
    var p = paddingStack[node.depth],
        x0 = node.x0 + p,
        y0 = node.y0 + p,
        x1 = node.x1 - p,
        y1 = node.y1 - p;
    if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
    if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
    node.x0 = x0;
    node.y0 = y0;
    node.x1 = x1;
    node.y1 = y1;
    if (node.children) {
      p = paddingStack[node.depth + 1] = paddingInner(node) / 2;
      x0 += paddingLeft(node) - p;
      y0 += paddingTop(node) - p;
      x1 -= paddingRight(node) - p;
      y1 -= paddingBottom(node) - p;
      if (x1 < x0) x0 = x1 = (x0 + x1) / 2;
      if (y1 < y0) y0 = y1 = (y0 + y1) / 2;
      tile(node, x0, y0, x1, y1);
    }
  }

  treemap.round = function(x) {
    return arguments.length ? (round = !!x, treemap) : round;
  };

  treemap.size = function(x) {
    return arguments.length ? (dx = +x[0], dy = +x[1], treemap) : [dx, dy];
  };

  treemap.tile = function(x) {
    return arguments.length ? (tile = required(x), treemap) : tile;
  };

  treemap.padding = function(x) {
    return arguments.length ? treemap.paddingInner(x).paddingOuter(x) : treemap.paddingInner();
  };

  treemap.paddingInner = function(x) {
    return arguments.length ? (paddingInner = typeof x === "function" ? x : constant(+x), treemap) : paddingInner;
  };

  treemap.paddingOuter = function(x) {
    return arguments.length ? treemap.paddingTop(x).paddingRight(x).paddingBottom(x).paddingLeft(x) : treemap.paddingTop();
  };

  treemap.paddingTop = function(x) {
    return arguments.length ? (paddingTop = typeof x === "function" ? x : constant(+x), treemap) : paddingTop;
  };

  treemap.paddingRight = function(x) {
    return arguments.length ? (paddingRight = typeof x === "function" ? x : constant(+x), treemap) : paddingRight;
  };

  treemap.paddingBottom = function(x) {
    return arguments.length ? (paddingBottom = typeof x === "function" ? x : constant(+x), treemap) : paddingBottom;
  };

  treemap.paddingLeft = function(x) {
    return arguments.length ? (paddingLeft = typeof x === "function" ? x : constant(+x), treemap) : paddingLeft;
  };

  return treemap;
}

function binary(parent, x0, y0, x1, y1) {
  var nodes = parent.children,
      i, n = nodes.length,
      sum, sums = new Array(n + 1);

  for (sums[0] = sum = i = 0; i < n; ++i) {
    sums[i + 1] = sum += nodes[i].value;
  }

  partition(0, n, parent.value, x0, y0, x1, y1);

  function partition(i, j, value, x0, y0, x1, y1) {
    if (i >= j - 1) {
      var node = nodes[i];
      node.x0 = x0, node.y0 = y0;
      node.x1 = x1, node.y1 = y1;
      return;
    }

    var valueOffset = sums[i],
        valueTarget = (value / 2) + valueOffset,
        k = i + 1,
        hi = j - 1;

    while (k < hi) {
      var mid = k + hi >>> 1;
      if (sums[mid] < valueTarget) k = mid + 1;
      else hi = mid;
    }

    if ((valueTarget - sums[k - 1]) < (sums[k] - valueTarget) && i + 1 < k) --k;

    var valueLeft = sums[k] - valueOffset,
        valueRight = value - valueLeft;

    if ((x1 - x0) > (y1 - y0)) {
      var xk = (x0 * valueRight + x1 * valueLeft) / value;
      partition(i, k, valueLeft, x0, y0, xk, y1);
      partition(k, j, valueRight, xk, y0, x1, y1);
    } else {
      var yk = (y0 * valueRight + y1 * valueLeft) / value;
      partition(i, k, valueLeft, x0, y0, x1, yk);
      partition(k, j, valueRight, x0, yk, x1, y1);
    }
  }
}

function sliceDice(parent, x0, y0, x1, y1) {
  (parent.depth & 1 ? treemapSlice : treemapDice)(parent, x0, y0, x1, y1);
}

var resquarify = (function custom(ratio) {

  function resquarify(parent, x0, y0, x1, y1) {
    if ((rows = parent._squarify) && (rows.ratio === ratio)) {
      var rows,
          row,
          nodes,
          i,
          j = -1,
          n,
          m = rows.length,
          value = parent.value;

      while (++j < m) {
        row = rows[j], nodes = row.children;
        for (i = row.value = 0, n = nodes.length; i < n; ++i) row.value += nodes[i].value;
        if (row.dice) treemapDice(row, x0, y0, x1, y0 += (y1 - y0) * row.value / value);
        else treemapSlice(row, x0, y0, x0 += (x1 - x0) * row.value / value, y1);
        value -= row.value;
      }
    } else {
      parent._squarify = rows = squarifyRatio(ratio, parent, x0, y0, x1, y1);
      rows.ratio = ratio;
    }
  }

  resquarify.ratio = function(x) {
    return custom((x = +x) > 1 ? x : 1);
  };

  return resquarify;
})(phi);

exports.cluster = cluster;
exports.hierarchy = hierarchy;
exports.pack = index;
exports.packSiblings = siblings;
exports.packEnclose = enclose;
exports.partition = partition;
exports.stratify = stratify;
exports.tree = tree;
exports.treemap = index$1;
exports.treemapBinary = binary;
exports.treemapDice = treemapDice;
exports.treemapSlice = treemapSlice;
exports.treemapSliceDice = sliceDice;
exports.treemapSquarify = squarify;
exports.treemapResquarify = resquarify;

Object.defineProperty(exports, '__esModule', { value: true });

})));


/***/ })
/******/ ]);