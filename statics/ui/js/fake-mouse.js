/*
 * Copyright (C) 2017 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

FakeMousePointer = function() {
  this.x = $(window).width() / 2;
  this.y = $(window).height() / 2;

  this.s = document.createElement('div');
  this.s.setAttribute('id', 'FakeMousePointer');
  this.s.setAttribute('style', 'position:absolute; left:' + this.x + 'px; top:' + this.y + 'px;');
  document.body.append(this.s);

  this.pointer = $('#FakeMousePointer');
};

FakeMousePointer.prototype.easeInOutQuart = function(t) { return t < 0.5 ? 8 * t * t * t * t : 1 - 8 * (--t) * t * t * t; };

FakeMousePointer.prototype.click = function(callback) {
	var self = this;

  var i = 4, down = false;
  var blink = function() {
    if (down) {
	    self.pointer.removeClass("mouse-down");
      down = false;
    } else {
    	self.pointer.addClass("mouse-down");
      down = true;
    }
    if (i-- > 0) {
      setTimeout(blink, 100);
    } else {
    	self.pointer.removeClass("mouse-down");
  		if (callback) callback();
    }
  };

  blink();
};

FakeMousePointer.prototype.moveTo = function(x, y, speed, callback) {
  var self = this;

  // move the pointer 1px away to allow selenium to click
  x += 1; y += 1;

  var path = document.createElementNS('http://www.w3.org/2000/svg','path');

  var x1 = this.x, y1 = this.y, x2 = x, y2 = y;
  var xx = x2 - x1, yy = y2 - y1;
  var l = Math.sqrt(xx * xx + yy * yy);
  var cx = (x1 + x2) / 2, cy  = (y1 + y2) / 2;
  var angle = Math.atan2(yy, xx);
  // curve height
  var dist = l * 0.17;
  var cpx = Math.sin(angle) * dist + cx;
  var cpy = -Math.cos(angle) * dist + cy;

  path.setAttribute('d','M ' + x1 + ' ' + y1 + 'Q '+ cpx + ' ' + cpy + ' ' + x2 + ' ' + y2);
  var len = path.getTotalLength();

  var move = function(percent, step) {
    var pt = path.getPointAtLength(percent / 100 * len);
    self.pointer.css({ left: pt.x, top: pt.y });

    percent += step;
    if (percent <= 100) {
      var ms = self.easeInOutQuart(percent/100);
			ms = ms * 5 + 5;

      setTimeout(function(){
        move(percent, speed);
      }, ms);
    } else {
      self.x = x;
      self.y = y;

			if (callback) callback();
    }
  };

  move(0, speed);
};

FakeMousePointer.prototype.clickTo = function(x, y, callback) {
	var self = this;

	this.moveTo(x, y, 2, function() { self.click(callback); });
};

FakeMousePointer.prototype.clickOn = function(el, callback) {
	var self = this;

  this.moveOn(el, function() {
    self.click(callback);
  });
};

FakeMousePointer.prototype.moveOn = function(el, callback) {
	var self = this;

  var br = el.getBoundingClientRect();
  this.moveTo((br.left + br.right) / 2, (br.top + br.bottom) / 2, 2, function() {
    // a last move to be as close as possible
    br = el.getBoundingClientRect();
    self.moveTo((br.left + br.right) / 2, (br.top + br.bottom) / 2, 10);

    if (callback) callback();
  });
};
