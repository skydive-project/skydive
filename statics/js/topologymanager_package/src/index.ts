import { GremlinFilteredTopologyDataSource, InfraTopologyDataSource, HostTopologyDataSource } from "./topologymanager/data_source/index";
import { SkydiveDefaultLayout, SkydiveInfraLayout, LayoutConfig } from "./topologymanager/topology_layout/index";
import * as events from 'events';

declare let window: any;
declare global {
    interface Window {
        d3: any;
        TopologyORegistry: any;
        detailedTopology: any;
        apiMixin: any;
        $: any;
        globalEventHandler: any;
        WebSocket: any;
    }
}
window.TopologyORegistry = {
    dataSources: {
        infraTopology: InfraTopologyDataSource,
        hostTopology: HostTopologyDataSource,
        filterTopologyByQuery: GremlinFilteredTopologyDataSource
    },
    layouts: {
        skydive_default: SkydiveDefaultLayout,
        infra: SkydiveInfraLayout
    },
    config: LayoutConfig,
    eventEmitter: events.EventEmitter
};
