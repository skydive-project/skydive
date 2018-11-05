import parseData, { parseSkydiveMessageWithOneNode, getNodeIDFromSkydiveMessageWithOneNode, getHostFromSkydiveMessageWithOneNode } from './parsers/index';

export default class DataManager {
    addNodeFromData(dataType: string, data: any) {
        parseSkydiveMessageWithOneNode(this, data);
    }
    removeAllNodesWhichBelongsToHostFromData(dataType: string, data: any): void {
        getHostFromSkydiveMessageWithOneNode(data);
    }
    updateFromData(dataType: string, data: any): void {
        parseData(this, dataType, data);
    }
    removeOldData(): void {
    }
}
