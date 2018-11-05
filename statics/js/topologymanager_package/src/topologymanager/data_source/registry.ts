import DataSourceI from "./interface";

export default class DataSourceRegistry {
    sources: Array<DataSourceI> = [];

    addSource(source: DataSourceI, defaultSource: boolean): void {
        this.sources.push(source);
    }
}
