import * as fs from 'fs';

class DataStore {
    mockDirectoryPath: String;

    constructor(mockDirectoryPath) {
        this.mockDirectoryPath = mockDirectoryPath;
    }

    getMock(mockName: any) {
        let json = JSON.parse(fs.readFileSync(
            this.mockDirectoryPath + mockName + '.json',
            'utf8'));
        return json;
    }
}

export default new DataStore('__tests__/mocks/');
