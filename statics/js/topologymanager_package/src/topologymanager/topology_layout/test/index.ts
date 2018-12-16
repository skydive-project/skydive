import { TopologyLayoutI } from "../index";
import LayoutConfig from '../../config';
import * as events from 'events';
import { LayoutBridgeUI, LayoutBridgeUII } from '../base/ui/index';
import { LayoutUI, EdgeUI, NodeUI, GroupUI } from '../base/ui/index';
import EventObserver from '../../event_observer';
import * as Simplify from '../../../simplify/index';
import { prepareDataForHierarchyLayout } from '../../../helpers';


class SimplifyDataHandler {
    data: any;

    constructor(data: any) {
        this.data = data;
    }

    simplify() {
        return new Promise((resolve: (msg: any) => void, reject: (msg: any) => void) => {
            this.mergeSimilarNodes()
                .then(this.flattenSamplesOfTopology.bind(this))
                .then(this.simplifyStructure.bind(this))
                .then(() => {
                    resolve(this.data);
                });
        });
    }

    simplifyStructure() {
        return new Promise((resolve: () => void, reject: (msg: any) => void) => {
            const strategy = Simplify.makeSimplificationStrategy("structure");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }

    mergeSimilarNodes() {
        return new Promise((resolve: () => void, reject: (msg: any) => void) => {
            const strategy = Simplify.makeSimplificationStrategy("merge_similar_nodes");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }

    flattenSamplesOfTopology() {
        return new Promise((resolve: () => void, reject: (msg: any) => void) => {
            const strategy = Simplify.makeSimplificationStrategy("flat_samples_of_topology");
            this.data = strategy.simplifyData("skydive", this.data);
            resolve();
        });
    }

}


export default class SkydiveTestLayout implements TopologyLayoutI {
    eventObserver: EventObserver = new EventObserver();
    uiBridge: LayoutBridgeUII;
    e: events.EventEmitter = new events.EventEmitter();
    alias: string = "test";
    active: boolean = false;
    config: LayoutConfig;
    selector: string;
    constructor(selector: string) {
        this.selector = selector;
        this.uiBridge = new LayoutBridgeUI(selector);
        this.uiBridge.useEventEmitter(this.e);
        this.e.on('node.selected', (d: any) => {
            // fix for compatibility with old skydive frontend
            if (!d.metadata) {
                d.metadata = d.Metadata;
            }
            this.notifyHandlers('nodeSelected', d);
        });
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useLayoutUI(new LayoutUI(selector));
        this.uiBridge.useNodeUI(new NodeUI());
        this.uiBridge.useGroupUI(new GroupUI());
        this.uiBridge.useEdgeUI(new EdgeUI());
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        this.active = true;
        console.log('current topology graph is ', window.topologyComponent.graph);
    }

    onSyncTopology(data: any) {
        const existsAnyVm = data.Nodes.some((n: any) => {
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
        simplifyHandler.simplify().then((data: any) => {
            console.log('After simplification', JSON.stringify(data));
            const hostID = data.Obj.Nodes.find((node: any) => { return node.Metadata.Type == 'host' }).ID;
            const topologyData = prepareDataForHierarchyLayout(data);
            this.uiBridge.start(topologyData, hostID);
        });

    }
    useConfig(config: LayoutConfig) {
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

    addHandler(...args: Array<any>) {
        this.eventObserver.addHandler.apply(this.eventObserver, args);
    }

    removeHandler(...args: Array<any>) {
        this.eventObserver.removeHandler.apply(this.eventObserver, args);
    }

    notifyHandlers(...args: Array<any>) {
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
