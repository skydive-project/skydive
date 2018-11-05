var getImagePath = function(label: string) {
    return 'statics/img/' + label + '.png';
};

export const minusImg = getImagePath('minus-outline-16');
export const plusImg = getImagePath('plus-16');
export const captureIndicatorImg = getImagePath('media-record');
export const pinIndicatorImg = getImagePath('pin');

var setupFixedImages = function(labelMap: any) {
    const imgMap: any = {};
    Object.keys(labelMap).forEach(function(key) {
        imgMap[key] = getImagePath(labelMap[key]);
    });
    return imgMap;
};

export const nodeImgMap = setupFixedImages({
    "host": "host",
    "port": "port",
    "ovsport": "port",
    "bridge": "bridge",
    "switch": "switch",
    "ovsbridge": "switch",
    "netns": "ns",
    "veth": "veth",
    "bond": "port",
    "default": "intf",
    // k8s
    "cluster": "cluster",
    "container": "container",
    "cronjob": "cronjob",
    "daemonset": "daemonset",
    "deployment": "deployment",
    "endpoints": "endpoints",
    "ingress": "ingress",
    "job": "job",
    "node": "host",
    "persistentvolume": "persistentvolume",
    "persistentvolumeclaim": "persistentvolumeclaim",
    "pod": "pod",
    "networkpolicy": "networkpolicy",
    "namespace": "ns",
    "replicaset": "replicaset",
    "replicationcontroller": "replicationcontroller",
    "service": "service",
    "statefulset": "statefulset",
    "storageclass": "storageclass",
    // istio
    "destinationrule": "destinationrule",
    "gateway": "gateway",
    "quotaspec": "quotaspec",
    "quotaspecbinding": "quotaspecbinding",
    "serviceentry": "serviceentry",
    "virtualservice": "virtualservice",
});

export const managerImgMap = setupFixedImages({
    "docker": "docker",
    "lxd": "lxd",
    "neutron": "openstack",
    "k8s": "k8s",
    "istio": "istio",
});
