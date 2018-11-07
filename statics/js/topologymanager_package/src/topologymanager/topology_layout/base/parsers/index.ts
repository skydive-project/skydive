import parseSkydiveData from './skydive';
import DataManager from '../data_manager';
export { parseSkydiveMessageWithOneNode, getNodeIDFromSkydiveMessageWithOneNode, parseSkydiveMessageWithOneNodeAndUpdateNode, getHostFromSkydiveMessageWithOneNode, getEdgeIDFromSkydiveMessageWithOneEdge, parseSkydiveMessageWithOneEdgeAndUpdateEdge, parseNewSkydiveEdgeAndUpdateDataManager } from './skydive';

export default function parseData(dataManager: DataManager, dataType: string, data: any): void {
    const parsers: any = {
        skydive: parseSkydiveData
    }
    if (!parsers[dataType]) {
        throw new Error("No registered parser for dataType " + dataType);
    }
    return parsers[dataType](dataManager, data);
}
