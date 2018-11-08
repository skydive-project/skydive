import BaseSkydiveDataSource from "./base_skydive_data_source";
import * as events from 'events';

export default class HostTopologyDataSource extends BaseSkydiveDataSource {
    dataSourceName: string = "host_topology";

    constructor(host: string) {
        super();
        this.filterQuery = "G.V().Has('Host', '" + host + "').SubGraph()";
    }

}
