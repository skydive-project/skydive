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
  "container": "docker",
  "node": "host",
  "pod": "pod",
  "networkpolicy": "networkpolicy",
  "namespace": "ns",
  "default": "intf",
});

var managerImgMap = setupFixedImages({
  "docker": "docker",
  "neutron": "openstack",
  "k8s": "k8s",
});
