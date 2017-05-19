
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

function bandwidthToString(kbps) {
  if (kbps >= 1000000)
    return (Math.floor(kbps / 1000000)).toString() + " Gbps";
  if (kbps >= 1000)
    return (Math.floor(kbps / 1000)).toString() + " Mbps";
  return kbps.toString() + " Kbps";
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

  await: function() {
    this._await = Array.prototype.slice.call(arguments);
    return this;
  }

};
