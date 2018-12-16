import { SkydiveTestLayout } from "./topologymanager/topology_layout/index";
import LayoutConfig from "./topologymanager/config";
import LayoutManager from "./topologymanager/manager";

declare let window: any;
declare global {
    interface Window {
        d3: any;
        TopologyORegistry: any;
        detailedTopology: any;
        apiMixin: any;
        $: any;
        WebSocket: any;
        topologyComponent: any;
        nodeImgMap: any;
    }
}
window.TopologyORegistry = {
    layouts: {
        test: SkydiveTestLayout
    },
    config: LayoutConfig,
    manager: LayoutManager
};
