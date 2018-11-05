import { DataSourceI, DataSourceRegistry } from "../../data_source/index";
import { TopologyLayoutI } from "../index";
import LayoutConfig from '../config';
import * as events from 'events';
import { DataManager } from '../base/index';
import { LayoutBridgeUI, LayoutBridgeUII } from '../base/ui/index';
export default class SkydiveInfraLayout implements TopologyLayoutI {
    uiBridge: LayoutBridgeUII;
    dataManager: DataManager = new DataManager();
    e: events.EventEmitter = new events.EventEmitter();
    alias: string = "skydive_infra";
    active: boolean = false;
    config: LayoutConfig;
    dataSources: DataSourceRegistry = new DataSourceRegistry();
    selector: string;
    constructor(selector: string) {
        this.selector = selector;
        this.uiBridge = new LayoutBridgeUI(selector);
        this.uiBridge.useEventEmitter(this.e);
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useDataManager(this.dataManager);
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        $(this.selector).empty();
        this.active = true;
        this.uiBridge.start();
    }
    useLinkLabelStrategy(linkLabelType: string) {
    }
    useConfig(config: LayoutConfig) {
        this.config = config;
        this.uiBridge.useConfig(this.config);
    }
    remove() {
        this.dataSources.sources.forEach((source: DataSourceI) => {
            source.unsubscribe();
        });
        this.active = false;
        this.uiBridge.remove();
        $(this.selector).empty();
    }
    addDataSource(dataSource: DataSourceI, defaultSource?: boolean) {
        this.dataSources.addSource(dataSource, !!defaultSource);
    }
    reactToDataSourceEvent(dataSource: DataSourceI, eventName: string, ...args: Array<any>) {
        console.log('Infra layout got an event', eventName, args);
    }
    reactToTheUiEvent(eventName: string, ...args: Array<any>) {
        this.e.emit('ui.' + eventName, ...args);
    }
}
