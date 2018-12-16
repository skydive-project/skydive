import * as events from 'events';
import LayoutContext from './layout_context';

export interface EdgeUII {
    createRoot(g: any, nodes?: any): void;
    root: any;
    gLink: any;
    tick(): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
    update(links?: any): void;
}

export interface EdgeUIConstructableI {
    new(): EdgeUII;
}

export class EdgeUI implements EdgeUII {
    gLink: any;
    root: any;
    layoutContext: LayoutContext;
    nodeIDToNode: any = {};

    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext
    }

    createRoot(g: any, nodes: any) {
        this.root = g;
        this.nodeIDToNode = nodes.reduce((accum: any, n: any) => {
            accum[n.ID] = n;
            return accum;
        }, {});
        this.gLink = g.append("g").attr('class', 'links').selectAll("line.link");
    }

    tick() {
        this.gLink.attr("d", (d: any) => {
            if (d.generateD3Path) {
                return d.generateD3Path().path;
            }
            // return '';
            let dModification = 'M ';
            let source = this.nodeIDToNode[d.source.data.ID];
            let target = this.nodeIDToNode[d.target.data.ID];
            // @todo  remove
            if (!source || !target) {
                console.log('Absent source or target.', d);
                return;
            }
            if (target.py && source.py && target.py < source.py) {
                const tmp = target;
                target = source;
                source = tmp;
            } else if (target.y < source.y) {
                const tmp = target;
                target = source;
                source = tmp;
            }
            const verticalIncrement = this.layoutContext.config.getValue('node.height') / 2;
            if (source.px && source.py) {
                dModification += source.px + " " + (source.py + verticalIncrement);
            } else {
                dModification += source.x + " " + (source.y + verticalIncrement);
            }
            dModification += " L "
            if (target.px && target.py) {
                dModification += target.px + " " + (target.py - verticalIncrement);
            } else {
                dModification += target.x + " " + (target.y - verticalIncrement);
            }
            return dModification;
        });

    }

    update(links: any) {
        // Update the links…
        this.gLink = this.gLink.data(links, function(d: any) { return d.id; });
        this.gLink.exit().remove();

        const linkEnter = this.gLink.enter()
            .append("path")
            .attr("id", function(d: any) { return d.generateD3Path ? Math.random() : "link-" + d.source.data.ID + "-" + d.target.data.ID; })
            .attr("class", (d: any) => { return this.layoutContext.config.getValue('link.className', d) })
            .style("stroke", (d: any) => { return this.layoutContext.config.getValue('link.color', d) });
        this.gLink = linkEnter.merge(this.gLink);
    }

}
