import * as events from 'events';
import LayoutConfig from '../../../config';
import LayoutContext from './layout_context';

export interface NodeUII {
    createRoot(g: any, nodes?: any): void;
    g: any;
    root: any;
    tick(): void;
    update(nodes?: any): void;
    layoutContext: LayoutContext;
    useLayoutContext(layoutContext: LayoutContext): void;
}

export interface NodeUIConstructableI {
    new(): NodeUII;
}


export class NodeUI implements NodeUII {
    g: any;
    layoutContext: LayoutContext;
    selectedNode: any;
    root: any;

    useLayoutContext(layoutContext: LayoutContext) {
        this.layoutContext = layoutContext;
    }

    createRoot(g: any, nodes: any) {
        this.root = g;
        this.g = g.append("g").attr('class', 'nodes').selectAll("circle.node");
    }

    tick() {
        this.g.attr("transform", function(d: any) {
            if (d.px && d.py) {
                return "translate(" + d.px + "," + d.py + ")";
            } else {

                return "translate(" + d.x + "," + d.y + ")";
            }
        });
    }

    update(nodes: any) {
        // Update the nodes…
        this.g = this.g.data(nodes, function(d: any) { return d.id; })
        this.g.exit().remove();
        const nodeEnter = this.g.enter()
            .append("g")
            .attr("class", (d: any) => {
                const classParts = ['node'];
                classParts.push(d.Metadata.Type);
                return classParts.join(' ');
            })
            .attr("id", function(d: any) { return "node-" + d.id; })
            .on("click", this.onNodeClick.bind(this))
            .on("mousedown", this.onMouseDown.bind(this))
            .call(window.d3.drag());

        nodeEnter.append("circle")
            .attr("r", this.layoutContext.config.getValue('node.radius'));

        // node picto
        nodeEnter.append("image")
            .attr("id", function(d: any) { return "node-img-" + d.id; })
            .attr("class", "picto")
            .attr("x", -12)
            .attr("y", -12)
            .attr("width", "24")
            .attr("height", "24")
            .attr("xlink:href", this.nodeImg.bind(this));

        // node title
        nodeEnter.append("text")
            .attr("dx", (d: any) => {
                return this.layoutContext.config.getValue('node.size', d) + 15;
            })
            .attr("dy", 10)
            .attr("class", "node-label")

            .text((d: any) => { return this.layoutContext.config.getValue('node.title', d); });

        // node type label
        nodeEnter.append("text")
            .attr("dx", (d: any) => { return this.layoutContext.config.getValue('node.typeLabelXShift', d) })
            .attr("dy", (d: any) => { return this.layoutContext.config.getValue('node.typeLabelYShift', d) })
            .attr("class", "type-label")
            .text((d: any) => { return this.layoutContext.config.getValue('node.typeLabel', d) });

        this.g = nodeEnter.merge(this.g);
    }

    nodeImg(d: any) {
        const t = d.Metadata.Type || "default";
        if (this.layoutContext.config.getValue('typesWithTextLabels').indexOf(t) !== -1) return null;
        return (t in window.nodeImgMap) ? window.nodeImgMap[t] : window.nodeImgMap["default"];
    }

    onNodeClick(d: any) {
        if (this.selectedNode === d) return;

        if (this.selectedNode) this.unselectNode(this.selectedNode);
        this.selectNode(d);

        // this.layoutContext.e.emit('node.selected', d);
    }

    onMouseDown(d: any) {
        this.onNodeClick(d);
    }

    selectNode(d: any) {
        var circle = this.root.select("#node-" + d.id)
            .select('circle');
        circle.transition().duration(500).attr('r', +circle.attr('r') + this.layoutContext.config.getValue('node.clickSizeDiff'));
        d.selected = true;
        this.selectedNode = d;
        this.updateGraph(d);
    }

    unselectNode(d: any) {
        var circle = this.root.select("#node-" + d.id)
            .select('circle');
        if (!circle) return;
        circle.transition().duration(500).attr('r', circle ? +circle.attr('r') - this.layoutContext.config.getValue('node.clickSizeDiff') : 0);
        d.selected = false;
        this.selectedNode = null;
        this.updateGraph(d);
    }

    updateGraph(d: any) {
        const nodeLabel = this.root.select("#node-" + d.id)
            .select('text.node-label');
        nodeLabel
            .attr("filter", d.selected ? "url(#selectedNodeFilter)" : null)
            .style("fill", this.layoutContext.config.getValue('node.labelColor'))
            .text((d: any) => { return this.layoutContext.config.getValue('node.title', d) });
        this.root.selectAll("g.node").sort(function(a: any, b: any) {
            if (a.id === d.id) return d.selected ? 1 : -1;
            if (b.id === d.id) return d.selected ? -1 : -1;
            return -1;
        });
    }

}
