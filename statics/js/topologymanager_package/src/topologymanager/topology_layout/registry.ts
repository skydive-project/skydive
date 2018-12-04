import TopologyLayoutI from './interface';

export default class LayoutRegistry {
    layouts: Array<TopologyLayoutI> = [];

    add(layout: TopologyLayoutI) {
        this.layouts.push(layout);
    }

    activate(layoutAlias: string) {
        this.layouts.forEach(l => {
            if (l.alias !== layoutAlias) {
                if (l.active) {
                    l.remove();
                }
                return;
            }
            l.initializer();
        });
    }

    getActive() {
        return this.layouts.find(l => {
            if (l.active) {
                return true;
            }
            return false;
        });
    }
}
