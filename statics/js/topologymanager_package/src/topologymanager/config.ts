const get = require('get-value')
const set = require('set-value');

export default class LayoutConfig {
    configuration: any;
    constructor(configuration: any) {
        this.configuration = configuration;
    }
    getValue(pathInConfig: String, ...args: any[]) {
        const val: any = get(this.configuration, pathInConfig);
        if (typeof val === 'function') {
            return val(...args)
        }
        return val;
    }
    setValue(pathInConfig: String, val: any) {
        set(this.configuration, pathInConfig, val);
    }
}
