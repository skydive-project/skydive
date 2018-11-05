import { Edge } from '../index';
import LayoutConfig from '../../../config';

export default function getLinkLabelRetrieveInformationStrategy(linkLabelType: string) {
    const typeToStrategy: any = {
        "latency": LatencyStrategy,
        "bandwidth": BandwidthStrategy
    }
    return new typeToStrategy[linkLabelType]();
}

const maxClockSkewMillis = 5 * 60 * 1000; // 5 minutes

function bandwidthToString(bps: any) {
    const KBPS = 1024, MBPS = 1024 * 1024, GBPS = 1024 * 1024 * 1024;
    if (bps >= GBPS)
        return (Math.floor(bps / GBPS)).toString() + " Gbps";
    if (bps >= MBPS)
        return (Math.floor(bps / MBPS)).toString() + " Mbps";
    if (bps >= KBPS)
        return (Math.floor(bps / KBPS)).toString() + " Kbps";
    return bps.toString() + " bps";
}


class LatencyStrategy {
    active: number;
    warning: number;
    alert: number;
    client: any = window.apiMixin;
    config: LayoutConfig;

    constructor() {
        this.client.created();
    }

    setup(config: LayoutConfig) {
        this.config = config;
        this.active = 0;
        this.warning = 10
        this.alert = 100;
    }

    updateLatency(edge: Edge, a: any, b: any) {
        edge.latencyTimestamp = Math.max(a.Last, b.Last);
        edge.latency = Math.abs(a.RTT - b.RTT) / 1000000;
    }

    flowQuery(nodeTID: string, trackingID: string, limit: number) {
        let has = `"NodeTID", ${nodeTID}`;
        if (typeof trackingID !== 'undefined') {
            has += `"TrackingID", ${trackingID}`;
        }
        has += `"RTT", NE(0)`;
        let query = `G.Flows().Has(${has}).Sort().Limit(${limit})`;
        return this.client.$topologyQuery(query);
    }

    flowQueryByNodeTID(nodeTID: string, limit: number) {
        return this.flowQuery(`"${nodeTID}"`, undefined, limit);
    }

    flowQueryByNodeTIDandTrackingID(nodeTID: string, flows: any) {
        let anyTrackingID = 'Within(';
        let i: any;
        for (i in flows) {
            const flow = flows[i];
            if (i != 0) {
                anyTrackingID += ', ';
            }
            anyTrackingID += `"${flow.TrackingID}"`;
        }
        anyTrackingID += ')';
        return this.flowQuery(`"${nodeTID}"`, anyTrackingID, 1);
    }

    flowCategoryKey(flow: any) {
        return `a=${flow.Link.A} b=${flow.Link.B} app=${flow.Application}`;
    }

    uniqueFlows(inFlows: any, count: number) {
        let outFlows = [];
        let hasCategory: any = {};
        for (let i in inFlows) {
            if (count <= 0) {
                break;
            }
            const flow = inFlows[i];
            const key = this.flowCategoryKey(flow);
            if (key in hasCategory) {
                continue;
            }
            hasCategory[key] = true;
            outFlows.push(flow);
            count--;
        }
        return outFlows;
    }

    mapFlowByTrackingID(flows: any) {
        let map: any = {};
        for (let i in flows) {
            const flow = flows[i];
            map[flow.TrackingID] = flow;
        }
        return map;
    }

    updateData(edge: Edge) {
        const a = edge.source.Metadata;
        const b = edge.target.Metadata;

        if (!a.Capture) {
            return;
        }
        if (!b.Capture) {
            return;
        }

        const maxFlows = 1000;
        this.flowQueryByNodeTID(a.TID, maxFlows)
            .then((aFlows: any) => {
                if (aFlows.length === 0) {
                    return;
                }
                const maxUniqueFlows = 100;
                aFlows = this.uniqueFlows(aFlows, maxUniqueFlows);
                const aFlowMap = this.mapFlowByTrackingID(aFlows);
                this.flowQueryByNodeTIDandTrackingID(b.TID, aFlows)
                    .then((bFlows: any) => {
                        if (bFlows.length === 0) {
                            return;
                        }
                        const bFlow = bFlows[0];
                        const aFlow = aFlowMap[bFlow.TrackingID];
                        this.updateLatency(edge, aFlow, bFlow);
                    })
                    .catch(function(error: any) {
                        console.log(error);
                    });
            })
            .catch(function(error: any) {
                console.log(error);
            });
    }

    hasData(edge: Edge) {
        if (!edge.latencyTimestamp) {
            return false;
        }
        const elapsedMillis = Date.now() - (+(new Date(edge.latencyTimestamp)));
        return elapsedMillis <= maxClockSkewMillis;
    }

    getText(edge: Edge) {
        return `${edge.latency} ms`;
    }

    isActive(edge: Edge) {
        return (edge.latency >= this.active) && (edge.latency < this.warning);
    }

    isWarning(edge: Edge) {
        return (edge.latency >= this.warning) && (edge.latency < this.alert);
    }

    isAlert(edge: Edge) {
        return (edge.latency >= this.alert);
    }
}

class BandwidthStrategy {
    client: any = window.apiMixin;
    config: any;

    constructor() {
        this.client.created();
    }

    bandwidthFromMetrics(metrics: any) {
        if (!metrics) {
            return 0;
        }
        if (!metrics.Last) {
            return 0;
        }
        if (!metrics.Start) {
            return 0;
        }

        const totalByte = (metrics.RxBytes || 0) + (metrics.TxBytes || 0);
        const deltaMillis = metrics.Last - metrics.Start;
        const elapsedMillis = Date.now() - (+(new Date(metrics.Last)));

        if (deltaMillis === 0) {
            return 0;
        }
        if (elapsedMillis > maxClockSkewMillis) {
            return 0;
        }

        return Math.floor(8 * totalByte * 1000 / deltaMillis); // bits-per-second
    }

    setup(config: LayoutConfig) {
        this.config = config;
    }

    updateData(edge: Edge) {
        var metadata;
        if (edge.target.Metadata.LastUpdateMetric) {
            metadata = edge.target.Metadata;
        } else if (edge.source.Metadata.LastUpdateMetric) {
            metadata = edge.source.Metadata;
        } else {
            return;
        }

        const defaultBandwidthBaseline = 1024 * 1024 * 1024; // 1 gbps

        edge.bandwidthBaseline = (this.config.getValue('bandwidth').bandwidthThreshold === 'relative') ?
            metadata.Speed || defaultBandwidthBaseline : 1;

        edge.bandwidthAbsolute = this.bandwidthFromMetrics(metadata.LastUpdateMetric);
        edge.bandwidth = edge.bandwidthAbsolute / edge.bandwidthBaseline;
    }

    hasData(edge: Edge) {
        if (!edge.target.Metadata.LastUpdateMetric && !edge.source.Metadata.LastUpdateMetric) {
            return false;
        }

        if (!edge.bandwidth) {
            return false;
        }
        return edge.bandwidth > this.config.getValue('bandwidth').active;
    }


    getText(edge: Edge) {
        return bandwidthToString(edge.bandwidthAbsolute);
    }

    isActive(edge: Edge) {
        return (edge.bandwidth > this.config.getValue('bandwidth').active) && (edge.bandwidth < this.config.getValue('bandwidth').warning);
    }

    isWarning(edge: Edge) {
        return (edge.bandwidth >= this.config.getValue('bandwidth').warning) && (edge.bandwidth < this.config.getValue('bandwidth').alert);
    }

    isAlert(edge: Edge) {
        return edge.bandwidth >= this.config.getValue('bandwidth').alert;
    }
}
