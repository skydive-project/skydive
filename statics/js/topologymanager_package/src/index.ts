import { LayoutConfig } from "./topologymanager/topology_layout/index";
import * as events from 'events';

declare let window: any;
declare global {
    interface Window {
        TopologyORegistry: any;
        detailedTopology: any;
    }
}
window.TopologyORegistry = {
    config: LayoutConfig
};
