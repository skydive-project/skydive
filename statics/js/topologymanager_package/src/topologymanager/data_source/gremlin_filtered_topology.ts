import BaseSkydiveDataSource from "./base_skydive_data_source";
import * as events from 'events';

export default class GremlinFilteredTopologyDataSource extends BaseSkydiveDataSource {
    dataSourceName: string = "gremlin_filtered__topology";

    constructor(topologyFilter: string) {
        super();
        this.filterQuery = topologyFilter;
    }

}
