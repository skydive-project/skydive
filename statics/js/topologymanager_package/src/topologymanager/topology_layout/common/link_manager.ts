import { IdAllocator } from './helpers';
import { HierarchicalGraph } from '../../../algorithms/index';

export default class LinkManager {

    dataKeeper: any = {};

    removeTopHierarchyLink(links: any) {
        return links.filter((link: any) => {
            return !(link.target.data.Type === "tophierarchy" || link.source.data.Type === "tophierarchy")
        });
    }

    removeLinksWithTheSameSourceAndTarget(links: any) {
        const currentlyExistingLinks = new Set([]);
        return links.filter((link: any) => {
            const linkId = link.target.data.ID + "-" + link.source.data.ID;
            if (currentlyExistingLinks.has(linkId)) {
                return false;
            }
            currentlyExistingLinks.add(linkId);
            return true;
        });
    }

    findCommonBuses(links: any) {
        const busesListToAdd: Array<LineWithCommonBus> = [];
        let source: any, target: any;
        const sourceToTargets: any = {};
        const targetToSources: any = {};
        links.forEach((link: any) => {
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target;
                target = link.source;
            } else {
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
        links.forEach((link: any) => {
            if (link instanceof LineWithCommonBus) {
                return;
            }
            let bus;
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target;
                target = link.source;
            } else {
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
            } else if (sources.size > 1) {
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
        links = links.filter((link: any) => {
            if (link instanceof LineWithCommonBus) {
                return true;
            }
            let buses;
            if (link.source.data.horizontal > link.target.data.horizontal) {
                source = link.target.data;
                target = link.source.data;
            } else {
                source = link.source.data;
                target = link.target.data;
            }
            const targets = sourceToTargets[source.ID];
            const sources = targetToSources[target.ID];
            if (targets.size > 1) {
                buses = source.buses;
            } else if (sources.size > 1) {
                buses = target.buses;
            }
            return !buses;
        });
        const matrixBusHandler = new MatrixBusHandler();
        busesListToAdd.forEach((bus: LineWithCommonBus) => {
            matrixBusHandler.addBus(bus);
        });
        busesListToAdd.forEach((bus: LineWithCommonBus) => {
            bus.matrixHandler = matrixBusHandler;
            bus.targets.sort((a: any, b: any) => {
                return a.vertical - b.vertical;
            })
            links.push(bus);
        });

        return links;
    }

    optimizeLinks(links: any) {
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
        return links
    }

}

export class MatrixBusHandler {
    all_buses: Array<LineWithCommonBus> = [];
    buses: any = {};
    pullOfColors: Set<String> = new Set([
        '#000080',
        '#FF00FF',
        '#800080',
        '#00FF00',
        '#008000',
        '#00FFFF',
        '#0000FF'
    ]);
    usedColorsPerHorizontal: Set<String> = new Set([]);
    yOffsets: any = {};
    calculatedBusesPerHorizontal: boolean = false;

    constructor() {
    }

    addBus(bus: LineWithCommonBus) {
        this.all_buses.push(bus);
    }

    chooseColorForHorizontal(horizontal: number) {
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

    getNumberOfBusesPerHorizontal(horizontal: any, node?: any) {
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

    getCurrentYOffsetPerHorizontal(width: number, horizontal: any) {
        this.calculateBusesPerHorizontal();
        if (!this.yOffsets[horizontal]) {
            this.yOffsets[horizontal] = 0
        }
        this.yOffsets[horizontal] += width / 2 / this.getNumberOfBusesPerHorizontal(horizontal);
        return this.yOffsets[horizontal];
    }
}

function generateConnectorToNode(node: any, connectorNodeCoordinates: any, generalBusY: any) {
    if (connectorNodeCoordinates.startsAtX === connectorNodeCoordinates.stopsAtX) {
        const parts: any = ['M ' + connectorNodeCoordinates.stopsAtX.toString() + ' ' + connectorNodeCoordinates.stopsAtY.toString() + ' L ' + connectorNodeCoordinates.startsAtX.toString() + ' ' + connectorNodeCoordinates.startsAtY.toString()];
        if (node.x !== connectorNodeCoordinates.startsAtX) {
            let nodeYCoord;
            if (node.y > connectorNodeCoordinates.startsAtY) {
                nodeYCoord = node.y - node.width;
            } else {
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
        const parts: any = ['M'];
        parts.push(node.x);
        if (node.y > connectorNodeCoordinates.startsAtY) {
            parts.push(node.y - node.width);
        } else {
            parts.push(node.y + node.width);
        }
        parts.push('L');
        if (connectorNodeCoordinates.startsAtX !== node.x) {
            parts.push(connectorNodeCoordinates.stopsAtX);
        } else {
            parts.push(connectorNodeCoordinates.startsAtX);
        }
        parts.push(generalBusY);
        parts.push('L');
        if (connectorNodeCoordinates.startsAtX !== node.x) {
            parts.push(connectorNodeCoordinates.startsAtX);
        } else {
            parts.push(connectorNodeCoordinates.stopsAtX);
        }
        parts.push(generalBusY);
        return parts.join(' ') + ' ';
    }
}


export class LineWithCommonBus {
    source: any;
    targets: Array<any> = [];
    matrixHandler: MatrixBusHandler;
    graph: HierarchicalGraph;
    private generatedD3Path: any;

    constructor(source: any) {
        this.source = source;
    }

    get horizontal() {
        if (this.targets[0].horizontal > this.source.horizontal) {
            return this.source.horizontal + 1;
        } else {
            return this.source.horizontal;
        }
    }

    addTarget(target: any) {
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
        const targetConnectors: Array<String> = [];
        let generalBusY: number;
        if (this.source.y > this.targets[0].y) {
            generalBusY = this.source.y - this.source.width - currentYOffset;
        } else {
            generalBusY = this.source.y + this.source.width + currentYOffset;
        }
        const coordinatesForTargets: any = [];
        const nodesInThisBus = this.targets.map(t => t.ID);
        nodesInThisBus.push(this.source.ID);
        this.targets.forEach((target: any) => {
            let xOffset = 0;
            if (this.source.y > target.y) {
                if (target.busyXCoordAtBottom === undefined) {
                    target.busyXCoordAtBottom = 0;
                } else {
                    target.busyXCoordAtBottom += this.source.width / 2 / 5;
                }
                xOffset = target.busyXCoordAtBottom;
            } else {
                if (target.busyXCoordAtTop === undefined) {
                    target.busyXCoordAtTop = 0;
                } else {
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
            let startFromY: number;
            if (this.source.y > target.y) {
                startFromY = target.y + target.width;
            } else {
                startFromY = target.y - target.width;
            }
            const stopAtX = startFromX;
            const stopAtY = generalBusY;
            coordinatesForTargets.push([startFromX, startFromY, stopAtX, stopAtY, target]);
        });
        coordinatesForTargets.sort((a: any, b: any) => {
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
        } else if (this.source.x > this.targets[0].x) {
            connectorToSourceStartsAtX = this.source.x;
            connectorToSourceStopsAtX = generalBusEndsAtX;
        } else {
            connectorToSourceStartsAtX = generalBusStartsFromX;
            connectorToSourceStopsAtX = this.source.x;
        }
        coordinatesForTargets.forEach((coords: any, targetNumber: number) => {
            const connectorToTarget = generateConnectorToNode(
                coords[4],
                {
                    startsAtX: coords[0],
                    startsAtY: coords[1],
                    stopsAtX: coords[2],
                    stopsAtY: coords[3]
                },
                generalBusY
            );
            targetConnectors.push(
                connectorToTarget
            );
        });
        const generalBus = 'M ' + generalBusStartsFromX.toString() + ' ' + generalBusY.toString() + ' L ' + generalBusEndsAtX.toString() + ' ' + generalBusY.toString();
        const connectorToSource = generateConnectorToNode(
            this.source,
            {
                startsAtX: connectorToSourceStartsAtX,
                startsAtY: connectorToSourceStartsAtY,
                stopsAtX: connectorToSourceStopsAtX,
                stopsAtY: connectorToSourceStopsAtY
            },
            generalBusY
        );
        this.generatedD3Path = {
            path: targetConnectors.join(' ') + ' ' + generalBus + ' ' + connectorToSource,
            color: this.matrixHandler.chooseColorForHorizontal(this.horizontal)
        }
        return this.generatedD3Path;
    }
}


export class BusesList {
    buses: Array<LineWithCommonBus> = [];

    findByHorizontal(source: any, target: any) {
        const horizontal = source.horizontal > target.horizontal ? source.horizontal : source.horizontal + 1;
        return this.buses.find((bus: LineWithCommonBus) => {
            const matches = horizontal === bus.horizontal;
            return matches;
        });
    }

    addBus(bus: LineWithCommonBus) {
        this.buses.push(bus);
    }

    getLength() {
        return this.buses.length;
    }

    getBusesOnHorizontal(horizontal: number) {
        return this.buses.filter((bus) => { return bus.horizontal === horizontal });
    }

}
