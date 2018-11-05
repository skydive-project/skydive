import { InfraTopologyDataSource, HostTopologyDataSource } from "./topologymanager/data_source/index";
import { SkydiveDefaultLayout, SkydiveInfraLayout, LayoutConfig } from "./topologymanager/topology_layout/index";
import * as events from 'events';

declare let window: any;
declare global {
    interface Window {
        TopologyORegistry: any;
        detailedTopology: any;
        websocket: any;
    }
}
window.TopologyORegistry = {
    dataSources: {
        infraTopology: InfraTopologyDataSource,
        hostTopology: HostTopologyDataSource
    },
    layouts: {
        skydive_default: SkydiveDefaultLayout,
        infra: SkydiveInfraLayout
    },
    config: LayoutConfig
};
