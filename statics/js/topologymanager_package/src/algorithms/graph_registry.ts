import { range } from '../iterator_helpers';

export class Graph {
    vertices: any = {};

    get depth() {
        return Math.max(...Object.keys(this.vertices).map((vId: any) => this.vertices[vId].getHorizontal() || 0));
    }

    get length() {
        const numberOfVerticesPerHorizontals: any = {};
        Object.keys(this.vertices).forEach((vId: any) => {
            if (!numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()]) {
                numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()] = 0;
            }
            ++numberOfVerticesPerHorizontals[this.vertices[vId].getHorizontal()];
        });
        return Math.max(...Object.keys(numberOfVerticesPerHorizontals).map((h: any) => numberOfVerticesPerHorizontals[h]));
    }

    addVertex(vertexdata: any) {
        this.vertices[vertexdata.ID] = new Vertex(vertexdata);
    }

    getVertexes(): Array<Vertex> {
        return Object.keys(this.vertices).map(vId => {
            return this.vertices[vId].getData();
        });
    }

    fixDepthForLastVertex(depth: number) {
        const graphDepth = this.depth;
        Object.keys(this.vertices).forEach(vId => {
            if (this.vertices[vId].getHorizontal() < graphDepth) {
                return;
            }
            this.vertices[vId].setHorizontal(depth);
        });
    }

    putVertexWithSpecificTypeToTheEndOfGraph(depth: number, t: string) {
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
        const currentVerticalPerHorizontal: any = {};
        while (true) {
            const foundAnyVertexWithNotDefinedVertical = Object.keys(this.vertices).some(vId => {
                return this.vertices[vId].getVertical() === undefined;
            });
            if (!foundAnyVertexWithNotDefinedVertical) {
                break;
            }
            range(this.depth, 0).forEach((h: any) => {
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
                    this.vertices[vId].getChildren().forEach((c: any) => {
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
                        } else {
                            this.setVerticalForVertex(c, currentVerticalPerHorizontal[h + 1], lengthPerVertex1);
                            if (c.getVertical() === undefined) {
                                return;
                            }
                        }
                        currentVerticalPerHorizontal[h + 1] = c.getVertical() + lengthPerVertex1;
                    });
                })
            });
        }
    }

    balanceForLength(length: number) {
        const k = this.length / length;
        Object.keys(this.vertices).forEach(vId => {
            // this.vertices[vId].setVertical(k * this.vertices[vId].getVertical());
        });
    }

    lengthForHorizontal(h: number) {
        return Object.keys(this.vertices).reduce((accum: number, vId: any) => {
            const v = this.vertices[vId];
            if (v.getHorizontal() !== h) {
                return accum;
            }
            ++accum;
            return accum;
        }, 0);
    }

    calculateCoordinatesForVertices(xMultiplier: number, yMultiplier: number, xOffset: number, yOffset: number) {
        Object.keys(this.vertices).forEach((vId: any) => {
            const v = this.vertices[vId];
            v.setXCoord(xMultiplier * v.getVertical() + xOffset);
            v.setYCoord(yMultiplier * v.getHorizontal() + yOffset);
        });
    }

    verticesIds(): Set<string> {
        return new Set(Object.keys(this.vertices));
    }

    intersects(graph: Graph): boolean {
        const verticesIds = this.verticesIds();
        const anotherVerticesIds = graph.verticesIds();
        const intersection = new Set(
            [...verticesIds].filter(x => anotherVerticesIds.has(x)));
        return !!intersection.size;
    }

    merge(graph: Graph) {
        this.vertices = Object.keys(graph.vertices).reduce((accum, vId) => {
            accum[vId] = graph.vertices[vId];
            return accum;
        }, this.vertices);
    }

    clearVerticals(vertices: any) {
        if (!vertices) {
            return;
        }
        vertices.forEach((v: any) => {
            this.vertices[v.ID].clearVertical();
            this.clearVerticals(this.vertices[v.ID].getChildren());
        });
    }

    clearVerticalsForHorizontal(h: number, vId: string, vertical: number) {
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

    isThisVerticalTakenOnHorizontal(horizontal: number, ignoreVId: string, verticalToLookFor: number, delta: number) {
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

    isXCoordBusy(x: number, minHorizontal: number, maxHorizontal: number, ignoreVIds: Array<string>) {
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

    suggestBetterFreeXCoord(x: number, minHorizontal: number, maxHorizontal: number) {
        return x + 10;
    }

    isVerticalTakenByAnyVertex(horizontal: number, verticalToCheck: number, delta: number) {
        const vertexes: Array<Vertex> = [];
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

    suggestVerticalToPlaceVertexOnHorizontal(vertex: Vertex, verticalToCheck: number, delta: number) {
        const horizontal = vertex.getHorizontal();
        let verticalToSuggest: any = null;
        const vertexes: Array<Vertex> = [];
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

    setVerticalForVertex(vertex: Vertex, vertical: number, delta: number) {
        const vertexPlacedOnThisVertical = this.isVerticalTakenByAnyVertex(vertex.getHorizontal(), vertical, delta);
        if (vertexPlacedOnThisVertical) {
            vertex.setVertical(this.suggestVerticalToPlaceVertexOnHorizontal(vertex, vertical, delta));
        } else {
            vertex.setVertical(vertical);
        }
        console.log('set vertical for node', vertex.getHorizontal(), vertex.getName(), vertical, vertex.getVertical(), delta);
    }

    setVerticalForFirstChildVertex(parentVertex: Vertex, vertex: Vertex, vertical: number, delta: number) {
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
        const theoreticalVerticals: Array<number> = [];
        let vert: any = null;
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
        } else {
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

export default class GraphRegistry {
    graphs: Array<Graph> = [];

    addGraph(graph: Graph) {
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

    getVertexes(): Array<Vertex> {
        return this.graphs.reduce((accum: Array<Vertex>, g: Graph) => {
            accum = accum.concat(g.getVertexes());
            return accum;
        }, []);
    }

    mergeGraphsWithCommonVertexes() {
        const mergedGraphsWithAnother: Array<number> = [];
        const addedGraphs: Array<number> = [];
        const mergedGraphs: Array<Graph> = [];
        this.graphs.forEach((g, i: number) => {
            if (mergedGraphsWithAnother.indexOf(i) !== -1) {
                return;
            }
            this.graphs.forEach((g1, j: number) => {
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


class Vertex {
    private data: any;

    constructor(data: any) {
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

    setHorizontal(depth: number) {
        this.data.depth = depth;
        this.data.horizontal = depth;
    }

    getVertical() {
        return this.data.vertical;
    }

    setVertical(vertical: number) {
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

    setYCoord(y: number) {
        this.data.y = y;
    }

    setXCoord(x: number) {
        this.data.x = x;
    }

    getXCoord() {
        return this.data.x;
    }

    setWidth(width: number) {
        this.data.width = width;
    }

    getChildren() {
        return this.data.children;
    }
}
