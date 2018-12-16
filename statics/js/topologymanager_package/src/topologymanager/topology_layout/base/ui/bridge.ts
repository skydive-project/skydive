import * as events from 'events';

import { LayoutUII, LayoutUI } from './layout';
import { NodeUII, NodeUI } from './node';
import { GroupUII, GroupUI } from './group';
import { EdgeUII, EdgeUI } from './link';
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';
import { FlattenNodesForHierarchy, LinkManager, LineWithCommonBus } from '../../common/index';


export interface LayoutBridgeUII {
    config: LayoutConfig;
    useEventEmitter(e: events.EventEmitter): void;
    layoutContext: LayoutContext;
    selector: string;
    layoutUI: LayoutUII;
    nodeUI: NodeUII;
    groupUI: GroupUII;
    edgeUI: EdgeUII;
    useLayoutUI(layoutUI: LayoutUII): void;
    useNodeUI(nodeUI: NodeUII): void;
    useGroupUI(groupUI: GroupUII): void;
    useEdgeUI(edgeUI: EdgeUII): void;
    useConfig(config: LayoutConfig): void;
    start(...args: Array<any>): void;
    remove(): void;
}

export interface LayoutBridgeUIConstructableI {
    new(selector: string): LayoutBridgeUII;
}

export class LayoutBridgeUI implements LayoutBridgeUII {
    edgeUI: EdgeUII;
    e: events.EventEmitter;
    selector: string;
    nodeUI: NodeUII;
    groupUI: GroupUII;
    layoutUI: LayoutUII;
    config: LayoutConfig;
    links: any;
    nodes: any;
    initialized: boolean = false;

    constructor(selector: string) {
        this.selector = selector;
    }

    useEventEmitter(e: events.EventEmitter) {
        this.e = e;
    }

    useNodeUI(nodeUI: NodeUII) {
        this.nodeUI = nodeUI;
    }

    useGroupUI(groupUI: GroupUII) {
        this.groupUI = groupUI;
    }

    useEdgeUI(edgeUI: EdgeUII) {
        this.edgeUI = edgeUI;
    }

    useConfig(config: LayoutConfig) {
        this.config = config;
    }

    useLayoutUI(layoutUI: LayoutUII) {
        this.layoutUI = layoutUI;
    }

    start(data: any, hostID: any) {
        this.initialized = false;
        this.layoutUI.useLayoutContext(this.layoutContext);
        this.layoutUI.createRoot();
        this.groupUI.useLayoutContext(this.layoutContext);
        this.groupUI.createRoot(this.layoutUI.g);
        this.layoutContext.subscribeToEvent('ui.tick', this.tick.bind(this));
        this.layoutContext.subscribeToEvent('ui.update', this.update.bind(this));

        const flattener = new FlattenNodesForHierarchy(this.config);
        let nodes = flattener.flattenForWidthAndHeight(hostID, data, this.layoutUI.width, this.layoutUI.height);
        const nodeIDToNode = nodes.reduce((acc: any, node: any) => {
            acc[node.ID] = node;
            return acc;
        }, {});
        nodes = Object.keys(nodeIDToNode).map(nodeId => nodeIDToNode[nodeId]);
        let links = window.d3.hierarchy(data).links();
        const linkManager = new LinkManager();
        console.log('Before links optimization', links.length);
        links = linkManager.optimizeLinks(links);
        links.forEach((link: any) => {
            if (!(link instanceof LineWithCommonBus)) {
                return;
            }
            link.graph = flattener.graph;
        })
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

    get layoutContext(): LayoutContext {
        const context = new LayoutContext();
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
