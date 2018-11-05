var getImagePath = function(label) {
	return 'statics/img/'+label+'.png';
};

var minusImg = getImagePath('minus-outline-16');
var plusImg = getImagePath('plus-16');
var captureIndicatorImg = getImagePath('media-record');
var pinIndicatorImg = getImagePath('pin');

var setupFixedImages = function(labelMap) {
  imgMap = {};
  Object.keys(labelMap).forEach(function(key) {
    imgMap[key] = getImagePath(labelMap[key]);
  });
  return imgMap;
};

var nodeImgMap = setupFixedImages({
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

var managerImgMap = setupFixedImages({
  "docker": "docker",
  "lxd": "lxd",
  "neutron": "openstack",
  "k8s": "k8s",
  "istio": "istio",
});
