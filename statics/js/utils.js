
function debounce(func, wait, immediate) {
	var timeout;
	return function() {
		var context = this, args = arguments;
		var later = function() {
			timeout = null;
			if (!immediate) func.apply(context, args);
		};
		var callNow = immediate && !timeout;
		clearTimeout(timeout);
		timeout = setTimeout(later, wait);
		if (callNow) func.apply(context, args);
	};
}

function bandwidthToString(bps) {
  const KBPS = 1024, MBPS = 1024*1024, GBPS = 1024*1024*1024;
  if (bps >= GBPS)
    return (Math.floor(bps / GBPS)).toString() + " Gbps";
  if (bps >= MBPS)
    return (Math.floor(bps / MBPS)).toString() + " Mbps";
  if (bps >= KBPS)
    return (Math.floor(bps / KBPS)).toString() + " Kbps";
  return bps.toString() + " bps";
}

function firstUppercase(string) {
  return string.charAt(0).toUpperCase() + string.slice(1);
}

var Queue = function() {
  this.calls = [];
  this.intervalID = null;
  this._await = null;
};

Queue.prototype = {

  defer: function() {
    this.calls.push(Array.prototype.slice.call(arguments));
  },

  start: function(interval) {
    var self = this;

    this.intervalID = setInterval(function() {
			var i, call, length = self.calls.length;
			for (i = 0; i != length; i++) {
				self.calls[i].shift().apply(null, self.calls[i]);
			}
			self.calls = [];

      if (self.await) {
        var fnc = self._await.shift();
        fnc.apply(null, self._await);
        self._await.unshift(fnc);
      }
    }, interval);
  },

  stop: function() {
    clearInterval(this.intervalID);
  },

	clear: function() {
	  this.calls = [];
	},

  await: function() {
    this._await = Array.prototype.slice.call(arguments);
    return this;
  }

};

function prettyBytes(value) {
	var g = Math.floor(value / 1000000000);
	var m = Math.floor((value - g * 1000000000) / 1000000);
	var k = Math.floor((value - g * 1000000000 - m * 1000000) / 1000);
	var b = value - g * 1000000000 - m * 1000000 - k * 1000;

	if (g) return g + "Gb (" + value.toLocaleString() + " bytes)";
	if (m) return m + "Mb (" + value.toLocaleString() + " bytes)";
	if (k) return k + "Kb (" + value.toLocaleString() + " bytes)";

	return b.toLocaleString() + " bytes";
}

_uniqueId = 0;
function uniqueId() {
	return "unique_id_" + _uniqueId;
}

function getNewUniqueId() {
	_uniqueId++;
	return uniqueId();
}
