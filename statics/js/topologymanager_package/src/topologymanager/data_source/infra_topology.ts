import BaseSkydiveDataSource from "./base_skydive_data_source";
import * as events from 'events';

export default class InfraTopologyDataSource extends BaseSkydiveDataSource {
    dataSourceName: string = "infra_topology";
    filterQuery: string = "G.V().Has('Type', 'host').SubGraph()";
}
