import { TopologyLayoutI } from "../index";
import LayoutConfig from '../../config';
import * as events from 'events';
import { LayoutBridgeUI, LayoutBridgeUII } from '../base/ui/index';
import { LayoutUI, EdgeUI, NodeUI, GroupUI } from '../base/ui/index';

export default class SkydiveTestLayout implements TopologyLayoutI {
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
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useLayoutUI(new LayoutUI(selector));
        this.uiBridge.useNodeUI(new NodeUI());
        this.uiBridge.useGroupUI(new GroupUI());
        this.uiBridge.useEdgeUI(new EdgeUI());
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        $(this.selector).empty();
        this.active = true;
        console.log('current topology graph is ', window.topologyComponent.graph);
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
}
