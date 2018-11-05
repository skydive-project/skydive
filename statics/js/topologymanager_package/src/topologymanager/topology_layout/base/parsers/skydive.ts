import DataManager from '../data_manager';


export default function parseSkydiveData(dataManager: DataManager, data: any): void {
    dataManager.removeOldData();
    console.log('Parse skydive data', data);
    data.Obj.Nodes.forEach((node: any) => {
    });
    data.Obj.Edges.forEach((edge: any) => {
    });
}

export function parseSkydiveMessageWithOneNode(dataManager: DataManager, data: any): void {
    console.log('Parse skydive message with one node', data);
}

export function getNodeIDFromSkydiveMessageWithOneNode(data: any): string {
    return data.Obj.ID;
}

export function getHostFromSkydiveMessageWithOneNode(data: any): string {
    return data.Obj;
}
