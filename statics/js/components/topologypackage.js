/******/ (function(modules) { // webpackBootstrap
/******/ 	// The module cache
/******/ 	var installedModules = {};
/******/
/******/ 	// The require function
/******/ 	function __webpack_require__(moduleId) {
/******/
/******/ 		// Check if module is in cache
/******/ 		if(installedModules[moduleId]) {
/******/ 			return installedModules[moduleId].exports;
/******/ 		}
/******/ 		// Create a new module (and put it into the cache)
/******/ 		var module = installedModules[moduleId] = {
/******/ 			i: moduleId,
/******/ 			l: false,
/******/ 			exports: {}
/******/ 		};
/******/
/******/ 		// Execute the module function
/******/ 		modules[moduleId].call(module.exports, module, module.exports, __webpack_require__);
/******/
/******/ 		// Flag the module as loaded
/******/ 		module.l = true;
/******/
/******/ 		// Return the exports of the module
/******/ 		return module.exports;
/******/ 	}
/******/
/******/
/******/ 	// expose the modules object (__webpack_modules__)
/******/ 	__webpack_require__.m = modules;
/******/
/******/ 	// expose the module cache
/******/ 	__webpack_require__.c = installedModules;
/******/
/******/ 	// define getter function for harmony exports
/******/ 	__webpack_require__.d = function(exports, name, getter) {
/******/ 		if(!__webpack_require__.o(exports, name)) {
/******/ 			Object.defineProperty(exports, name, {
/******/ 				configurable: false,
/******/ 				enumerable: true,
/******/ 				get: getter
/******/ 			});
/******/ 		}
/******/ 	};
/******/
/******/ 	// getDefaultExport function for compatibility with non-harmony modules
/******/ 	__webpack_require__.n = function(module) {
/******/ 		var getter = module && module.__esModule ?
/******/ 			function getDefault() { return module['default']; } :
/******/ 			function getModuleExports() { return module; };
/******/ 		__webpack_require__.d(getter, 'a', getter);
/******/ 		return getter;
/******/ 	};
/******/
/******/ 	// Object.prototype.hasOwnProperty.call
/******/ 	__webpack_require__.o = function(object, property) { return Object.prototype.hasOwnProperty.call(object, property); };
/******/
/******/ 	// __webpack_public_path__
/******/ 	__webpack_require__.p = "";
/******/
/******/ 	// Load entry module and return exports
/******/ 	return __webpack_require__(__webpack_require__.s = 13);
/******/ })
/************************************************************************/
/******/ ([
/* 0 */
/***/ (function(module, exports) {

// Copyright Joyent, Inc. and other Node contributors.
//
// Permission is hereby granted, free of charge, to any person obtaining a
// copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to permit
// persons to whom the Software is furnished to do so, subject to the
// following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS
// OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN
// NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
// DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR
// OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE
// USE OR OTHER DEALINGS IN THE SOFTWARE.

function EventEmitter() {
  this._events = this._events || {};
  this._maxListeners = this._maxListeners || undefined;
}
module.exports = EventEmitter;

// Backwards-compat with node 0.10.x
EventEmitter.EventEmitter = EventEmitter;

EventEmitter.prototype._events = undefined;
EventEmitter.prototype._maxListeners = undefined;

// By default EventEmitters will print a warning if more than 10 listeners are
// added to it. This is a useful default which helps finding memory leaks.
EventEmitter.defaultMaxListeners = 10;

// Obviously not all Emitters should be limited to 10. This function allows
// that to be increased. Set to zero for unlimited.
EventEmitter.prototype.setMaxListeners = function(n) {
  if (!isNumber(n) || n < 0 || isNaN(n))
    throw TypeError('n must be a positive number');
  this._maxListeners = n;
  return this;
};

EventEmitter.prototype.emit = function(type) {
  var er, handler, len, args, i, listeners;

  if (!this._events)
    this._events = {};

  // If there is no 'error' event listener then throw.
  if (type === 'error') {
    if (!this._events.error ||
        (isObject(this._events.error) && !this._events.error.length)) {
      er = arguments[1];
      if (er instanceof Error) {
        throw er; // Unhandled 'error' event
      } else {
        // At least give some kind of context to the user
        var err = new Error('Uncaught, unspecified "error" event. (' + er + ')');
        err.context = er;
        throw err;
      }
    }
  }

  handler = this._events[type];

  if (isUndefined(handler))
    return false;

  if (isFunction(handler)) {
    switch (arguments.length) {
      // fast cases
      case 1:
        handler.call(this);
        break;
      case 2:
        handler.call(this, arguments[1]);
        break;
      case 3:
        handler.call(this, arguments[1], arguments[2]);
        break;
      // slower
      default:
        args = Array.prototype.slice.call(arguments, 1);
        handler.apply(this, args);
    }
  } else if (isObject(handler)) {
    args = Array.prototype.slice.call(arguments, 1);
    listeners = handler.slice();
    len = listeners.length;
    for (i = 0; i < len; i++)
      listeners[i].apply(this, args);
  }

  return true;
};

EventEmitter.prototype.addListener = function(type, listener) {
  var m;

  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  if (!this._events)
    this._events = {};

  // To avoid recursion in the case that type === "newListener"! Before
  // adding it to the listeners, first emit "newListener".
  if (this._events.newListener)
    this.emit('newListener', type,
              isFunction(listener.listener) ?
              listener.listener : listener);

  if (!this._events[type])
    // Optimize the case of one listener. Don't need the extra array object.
    this._events[type] = listener;
  else if (isObject(this._events[type]))
    // If we've already got an array, just append.
    this._events[type].push(listener);
  else
    // Adding the second element, need to change to array.
    this._events[type] = [this._events[type], listener];

  // Check for listener leak
  if (isObject(this._events[type]) && !this._events[type].warned) {
    if (!isUndefined(this._maxListeners)) {
      m = this._maxListeners;
    } else {
      m = EventEmitter.defaultMaxListeners;
    }

    if (m && m > 0 && this._events[type].length > m) {
      this._events[type].warned = true;
      console.error('(node) warning: possible EventEmitter memory ' +
                    'leak detected. %d listeners added. ' +
                    'Use emitter.setMaxListeners() to increase limit.',
                    this._events[type].length);
      if (typeof console.trace === 'function') {
        // not supported in IE 10
        console.trace();
      }
    }
  }

  return this;
};

EventEmitter.prototype.on = EventEmitter.prototype.addListener;

EventEmitter.prototype.once = function(type, listener) {
  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  var fired = false;

  function g() {
    this.removeListener(type, g);

    if (!fired) {
      fired = true;
      listener.apply(this, arguments);
    }
  }

  g.listener = listener;
  this.on(type, g);

  return this;
};

// emits a 'removeListener' event iff the listener was removed
EventEmitter.prototype.removeListener = function(type, listener) {
  var list, position, length, i;

  if (!isFunction(listener))
    throw TypeError('listener must be a function');

  if (!this._events || !this._events[type])
    return this;

  list = this._events[type];
  length = list.length;
  position = -1;

  if (list === listener ||
      (isFunction(list.listener) && list.listener === listener)) {
    delete this._events[type];
    if (this._events.removeListener)
      this.emit('removeListener', type, listener);

  } else if (isObject(list)) {
    for (i = length; i-- > 0;) {
      if (list[i] === listener ||
          (list[i].listener && list[i].listener === listener)) {
        position = i;
        break;
      }
    }

    if (position < 0)
      return this;

    if (list.length === 1) {
      list.length = 0;
      delete this._events[type];
    } else {
      list.splice(position, 1);
    }

    if (this._events.removeListener)
      this.emit('removeListener', type, listener);
  }

  return this;
};

EventEmitter.prototype.removeAllListeners = function(type) {
  var key, listeners;

  if (!this._events)
    return this;

  // not listening for removeListener, no need to emit
  if (!this._events.removeListener) {
    if (arguments.length === 0)
      this._events = {};
    else if (this._events[type])
      delete this._events[type];
    return this;
  }

  // emit removeListener for all listeners on all events
  if (arguments.length === 0) {
    for (key in this._events) {
      if (key === 'removeListener') continue;
      this.removeAllListeners(key);
    }
    this.removeAllListeners('removeListener');
    this._events = {};
    return this;
  }

  listeners = this._events[type];

  if (isFunction(listeners)) {
    this.removeListener(type, listeners);
  } else if (listeners) {
    // LIFO order
    while (listeners.length)
      this.removeListener(type, listeners[listeners.length - 1]);
  }
  delete this._events[type];

  return this;
};

EventEmitter.prototype.listeners = function(type) {
  var ret;
  if (!this._events || !this._events[type])
    ret = [];
  else if (isFunction(this._events[type]))
    ret = [this._events[type]];
  else
    ret = this._events[type].slice();
  return ret;
};

EventEmitter.prototype.listenerCount = function(type) {
  if (this._events) {
    var evlistener = this._events[type];

    if (isFunction(evlistener))
      return 1;
    else if (evlistener)
      return evlistener.length;
  }
  return 0;
};

EventEmitter.listenerCount = function(emitter, type) {
  return emitter.listenerCount(type);
};

function isFunction(arg) {
  return typeof arg === 'function';
}

function isNumber(arg) {
  return typeof arg === 'number';
}

function isObject(arg) {
  return typeof arg === 'object' && arg !== null;
}

function isUndefined(arg) {
  return arg === void 0;
}


/***/ }),
/* 1 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__registry__ = __webpack_require__(16);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__registry__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__infra_topology__ = __webpack_require__(17);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return __WEBPACK_IMPORTED_MODULE_1__infra_topology__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__host_topology__ = __webpack_require__(18);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return __WEBPACK_IMPORTED_MODULE_2__host_topology__["a"]; });





/***/ }),
/* 2 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * isobject <https://github.com/jonschlinkert/isobject>
 *
 * Copyright (c) 2014-2015, Jon Schlinkert.
 * Licensed under the MIT License.
 */



var isArray = __webpack_require__(22);

module.exports = function isObject(val) {
  return val != null && typeof val === 'object' && isArray(val) === false;
};


/***/ }),
/* 3 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__data_manager__ = __webpack_require__(26);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__data_manager__["a"]; });



/***/ }),
/* 4 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__registry__ = __webpack_require__(27);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__registry__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__node__ = __webpack_require__(5);
/* unused harmony reexport Node */




/***/ }),
/* 5 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__edge_index__ = __webpack_require__(6);

class Node {
    constructor() {
        this.selected = false;
        this.fx = null;
        this.fy = null;
        this.group = null;
        this.emphasized = false;
        this.highlighted = false;
        this.fixed = false;
        this.visible = false;
        this.edges = new __WEBPACK_IMPORTED_MODULE_0__edge_index__["a" /* EdgeRegistry */]();
    }
    static createFromData(ID, Name, Host, Metadata) {
        const node = new Node();
        node.ID = ID;
        node.Name = Name;
        node.Host = Host;
        node.Metadata = Metadata;
        return node;
    }
    get id() {
        return this.ID;
    }
    equalsTo(d) {
        return d.ID == this.ID;
    }
    hasType(Type) {
        return this.Metadata.Type === Type;
    }
    d3_id() {
        return this.id;
    }
    isGroupOwner(group, Type) {
        return this.group && this.group.owner.equalsTo(this) && (!Type || Type === this.group.Type);
    }
    clone() {
        return Node.createFromData(this.ID, this.Name, this.Host, this.Metadata);
    }
    isCaptureOn() {
        return "Capture/id" in this.Metadata;
    }
    isCaptureAllowed() {
        const allowedTypes = ["device", "veth", "ovsbridge", "geneve", "vlan", "bond", "ovsport",
            "internal", "tun", "bridge", "vxlan", "gre", "gretap", "dpdkport"];
        return allowedTypes.indexOf(this.Metadata.Type) >= 0;
    }
    getD3XCoord() {
        return this.x;
    }
    getD3YCoord() {
        return this.y;
    }
    onTheScreen() {
        return !!(this.x && this.y);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = Node;



/***/ }),
/* 6 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__registry__ = __webpack_require__(28);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__registry__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__edge__ = __webpack_require__(7);
/* unused harmony reexport Edge */




/***/ }),
/* 7 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class Edge {
    constructor() {
        this.selected = false;
    }
    static createFromData(ID, Host, Metadata, source, target) {
        const edge = new Edge();
        edge.ID = ID;
        edge.Host = Host;
        edge.source = source;
        edge.target = target;
        edge.Metadata = Metadata;
        return edge;
    }
    get id() {
        return this.ID;
    }
    hasRelationType(relationType) {
        return this.Metadata.RelationType === relationType;
    }
    hasType(Type) {
        return this.Metadata.Type === Type;
    }
    d3_id() {
        return this.ID;
    }
    equalsTo(compareTo) {
        return compareTo.ID === this.ID;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = Edge;



/***/ }),
/* 8 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__group__ = __webpack_require__(9);

function fixDepthAndLevelForGroup(g, level = 0) {
    const group = g;
    while (g) {
        if (level > g.depth)
            g.depth = level;
        level++;
        g = g.parent;
    }
    group.level = level;
}
class GroupRegistry {
    constructor() {
        this.groups = [];
    }
    addGroupFromData(owner, Type) {
        const g = __WEBPACK_IMPORTED_MODULE_0__group__["a" /* default */].createFromData(owner, Type);
        this.groups.push(g);
        return g;
    }
    getGroupByOwner(owner) {
        return this.groups.find((g) => g.owner.equalsTo(owner));
    }
    getGroupByOwnerId(ownerId) {
        return this.groups.find((g) => g.owner.ID == ownerId);
    }
    removeById(ID) {
        this.groups = this.groups.filter((g) => g.ID == ID);
    }
    addGroup(group) {
        this.groups.push(group);
    }
    updateLevelAndDepth(collapseLevel = 0, isAutoExpand = false) {
        this.groups.forEach((g) => {
            fixDepthAndLevelForGroup(g);
        });
        this.groups.forEach((g) => {
            if (g.level > collapseLevel && isAutoExpand === false) {
                return;
            }
            if (isAutoExpand) {
                g.collapsed = !isAutoExpand;
                return;
            }
            if (collapseLevel >= g.level) {
                g.collapsed = false;
                return;
            }
        });
        this.groups.sort(function (a, b) { return a.level - b.level; });
    }
    getGroupsWithNoParent() {
        return this.groups.filter((g) => {
            return !!g.parent;
        });
    }
    get size() {
        return this.groups.length;
    }
    removeOldData() {
        this.groups = [];
    }
    getVisibleGroups(visibilityLevel, autoExpand) {
        return this.groups.filter((group) => {
            if (autoExpand) {
                return true;
            }
            if (group.level > visibilityLevel) {
                if (group.level === visibilityLevel + 1) {
                    return true;
                }
                if (!group.collapsed) {
                    return true;
                }
                return false;
            }
            return true;
        });
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = GroupRegistry;



/***/ }),
/* 9 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__node_index__ = __webpack_require__(4);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__registry__ = __webpack_require__(8);


class Group {
    constructor() {
        this.members = new __WEBPACK_IMPORTED_MODULE_0__node_index__["a" /* NodeRegistry */]();
        this.children = new __WEBPACK_IMPORTED_MODULE_1__registry__["a" /* default */]();
        this.level = 1;
        this.depth = 1;
        this.collapsed = true;
        this.d = "";
    }
    // collapseLinks: EdgeRegistry = new EdgeRegistry();
    static createFromData(owner, Type) {
        const group = new Group();
        group.owner = owner;
        group.Type = Type;
        group.ID = Group.currentGroupId;
        ++Group.currentGroupId;
        return group;
    }
    setParent(parent) {
        this.parent = parent;
    }
    addMember(node) {
        this.members.addNode(node);
    }
    delMember(node) {
        this.members.removeNodeByID(node.id);
    }
    isEqualTo(group) {
        return this.ID === group.ID;
    }
    d3_id() {
        return this.ID;
    }
    collapse() {
        this.collapsed = true;
    }
    uncollapse() {
        this.collapsed = false;
    }
    hasOutsideLink() {
        return !!this.members.nodes.some((n) => {
            const edges = n.edges;
            return edges.edges.some((e) => {
                if (e.Metadata.RelationType !== "ownership" && !e.source.group.isEqualTo(e.target.group))
                    return true;
            });
        });
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = Group;

Group.currentGroupId = 1;


/***/ }),
/* 10 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["a"] = parseSkydiveData;
/* harmony export (immutable) */ __webpack_exports__["d"] = parseSkydiveMessageWithOneNode;
/* harmony export (immutable) */ __webpack_exports__["c"] = getNodeIDFromSkydiveMessageWithOneNode;
/* harmony export (immutable) */ __webpack_exports__["e"] = parseSkydiveMessageWithOneNodeAndUpdateNode;
/* harmony export (immutable) */ __webpack_exports__["b"] = getHostFromSkydiveMessageWithOneNode;
function proceedNewEdge(dataManager, e) {
    e.source.edges.addEdge(e);
    e.target.edges.addEdge(e);
    if (e.Metadata.RelationType == "ownership" || e.Metadata.Type === "vlan") {
        let group = dataManager.groupManager.getGroupByOwner(e.source);
        if (!group) {
            const groupType = "ownership";
            group = dataManager.groupManager.addGroupFromData(e.source, groupType);
            if (e.source.group) {
                e.source.group.delMember(e.source);
                group.setParent(e.source.group);
            }
            e.source.group = group;
            group.addMember(e.source);
        }
        const tg = dataManager.groupManager.getGroupByOwner(e.target);
        if (tg) {
            if (!tg.parent) {
                group.delMember(e.target);
                tg.setParent(group);
            }
            else if (!tg.parent.isEqualTo(group)) {
                group.delMember(e.target);
                tg.setParent(group);
            }
        }
        if (!e.target.isGroupOwner()) {
            e.target.group = group;
            group.addMember(e.target);
        }
    }
}
function parseSkydiveData(dataManager, data) {
    dataManager.removeOldData();
    console.log('Parse skydive data', data);
    data.Obj.Nodes.forEach((node) => {
        dataManager.nodeManager.addNodeFromData(node.ID, node.Metadata.Name, node.Host, node.Metadata);
    });
    data.Obj.Edges.forEach((edge) => {
        dataManager.edgeManager.addEdgeFromData(edge.ID, edge.Host, edge.Metadata, dataManager.nodeManager.getNodeById(edge.Parent), dataManager.nodeManager.getNodeById(edge.Child));
    });
    const ownershipEdges = dataManager.edgeManager.getEdgesWithRelationType("ownership");
    ownershipEdges.forEach((e) => {
        proceedNewEdge(dataManager, e);
    });
    const layer2Edges = dataManager.edgeManager.getEdgesWithRelationType("layer2");
    layer2Edges.forEach((e) => {
        proceedNewEdge(dataManager, e);
    });
    dataManager.groupManager.groups.forEach((g) => {
        if (!g.parent) {
            return;
        }
        g.parent.children.addGroup(g);
    });
    dataManager.groupManager.updateLevelAndDepth(dataManager.layoutContext.collapseLevel, dataManager.layoutContext.isAutoExpand());
    const hostToNode = dataManager.nodeManager.nodes.reduce((accum, n) => {
        if (!n.hasType("host")) {
            return accum;
        }
        accum[n.Name] = n;
        return accum;
    }, {});
    // normalize hosts, it always should be kind of group
    dataManager.nodeManager.nodes.forEach((n) => {
        if (!n.hasType("host")) {
            if (!n.group) {
                const hostNode = hostToNode[n.Host];
                hostNode.group.addMember(n);
            }
            return;
        }
        if (n.group) {
            return;
        }
        const groupType = "ownership";
        const group = dataManager.groupManager.addGroupFromData(n, groupType);
        n.group = group;
        group.addMember(n);
    });
}
function parseSkydiveMessageWithOneNode(dataManager, data) {
    console.log('Parse skydive message with one node', data);
    dataManager.nodeManager.addNodeFromData(data.Obj.ID, data.Obj.Metadata.Name, data.Obj.Host, data.Obj.Metadata);
}
function getNodeIDFromSkydiveMessageWithOneNode(data) {
    return data.Obj.ID;
}
function parseSkydiveMessageWithOneNodeAndUpdateNode(node, data) {
    node.Name = data.Obj.Name;
    node.Host = data.Obj.Host;
    node.Metadata = data.Obj.Metadata;
}
function getHostFromSkydiveMessageWithOneNode(data) {
    return data.Obj;
}


/***/ }),
/* 11 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__bridge__ = __webpack_require__(31);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__bridge__["a"]; });



/***/ }),
/* 12 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__strategy__ = __webpack_require__(33);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__strategy__["a"]; });



/***/ }),
/* 13 */
/***/ (function(module, exports, __webpack_require__) {

__webpack_require__(14);
module.exports = __webpack_require__(15);


/***/ }),
/* 14 */
/***/ (function(module, exports) {

(function (global) {
  var babelHelpers = global.babelHelpers = {};
  babelHelpers.typeof = typeof Symbol === "function" && typeof Symbol.iterator === "symbol" ? function (obj) {
    return typeof obj;
  } : function (obj) {
    return obj && typeof Symbol === "function" && obj.constructor === Symbol && obj !== Symbol.prototype ? "symbol" : typeof obj;
  };

  babelHelpers.jsx = function () {
    var REACT_ELEMENT_TYPE = typeof Symbol === "function" && Symbol.for && Symbol.for("react.element") || 0xeac7;
    return function createRawReactElement(type, props, key, children) {
      var defaultProps = type && type.defaultProps;
      var childrenLength = arguments.length - 3;

      if (!props && childrenLength !== 0) {
        props = {};
      }

      if (props && defaultProps) {
        for (var propName in defaultProps) {
          if (props[propName] === void 0) {
            props[propName] = defaultProps[propName];
          }
        }
      } else if (!props) {
        props = defaultProps || {};
      }

      if (childrenLength === 1) {
        props.children = children;
      } else if (childrenLength > 1) {
        var childArray = Array(childrenLength);

        for (var i = 0; i < childrenLength; i++) {
          childArray[i] = arguments[i + 3];
        }

        props.children = childArray;
      }

      return {
        $$typeof: REACT_ELEMENT_TYPE,
        type: type,
        key: key === undefined ? null : '' + key,
        ref: null,
        props: props,
        _owner: null
      };
    };
  }();

  babelHelpers.asyncIterator = function (iterable) {
    if (typeof Symbol === "function") {
      if (Symbol.asyncIterator) {
        var method = iterable[Symbol.asyncIterator];
        if (method != null) return method.call(iterable);
      }

      if (Symbol.iterator) {
        return iterable[Symbol.iterator]();
      }
    }

    throw new TypeError("Object is not async iterable");
  };

  babelHelpers.asyncGenerator = function () {
    function AwaitValue(value) {
      this.value = value;
    }

    function AsyncGenerator(gen) {
      var front, back;

      function send(key, arg) {
        return new Promise(function (resolve, reject) {
          var request = {
            key: key,
            arg: arg,
            resolve: resolve,
            reject: reject,
            next: null
          };

          if (back) {
            back = back.next = request;
          } else {
            front = back = request;
            resume(key, arg);
          }
        });
      }

      function resume(key, arg) {
        try {
          var result = gen[key](arg);
          var value = result.value;

          if (value instanceof AwaitValue) {
            Promise.resolve(value.value).then(function (arg) {
              resume("next", arg);
            }, function (arg) {
              resume("throw", arg);
            });
          } else {
            settle(result.done ? "return" : "normal", result.value);
          }
        } catch (err) {
          settle("throw", err);
        }
      }

      function settle(type, value) {
        switch (type) {
          case "return":
            front.resolve({
              value: value,
              done: true
            });
            break;

          case "throw":
            front.reject(value);
            break;

          default:
            front.resolve({
              value: value,
              done: false
            });
            break;
        }

        front = front.next;

        if (front) {
          resume(front.key, front.arg);
        } else {
          back = null;
        }
      }

      this._invoke = send;

      if (typeof gen.return !== "function") {
        this.return = undefined;
      }
    }

    if (typeof Symbol === "function" && Symbol.asyncIterator) {
      AsyncGenerator.prototype[Symbol.asyncIterator] = function () {
        return this;
      };
    }

    AsyncGenerator.prototype.next = function (arg) {
      return this._invoke("next", arg);
    };

    AsyncGenerator.prototype.throw = function (arg) {
      return this._invoke("throw", arg);
    };

    AsyncGenerator.prototype.return = function (arg) {
      return this._invoke("return", arg);
    };

    return {
      wrap: function (fn) {
        return function () {
          return new AsyncGenerator(fn.apply(this, arguments));
        };
      },
      await: function (value) {
        return new AwaitValue(value);
      }
    };
  }();

  babelHelpers.asyncGeneratorDelegate = function (inner, awaitWrap) {
    var iter = {},
        waiting = false;

    function pump(key, value) {
      waiting = true;
      value = new Promise(function (resolve) {
        resolve(inner[key](value));
      });
      return {
        done: false,
        value: awaitWrap(value)
      };
    }

    ;

    if (typeof Symbol === "function" && Symbol.iterator) {
      iter[Symbol.iterator] = function () {
        return this;
      };
    }

    iter.next = function (value) {
      if (waiting) {
        waiting = false;
        return value;
      }

      return pump("next", value);
    };

    if (typeof inner.throw === "function") {
      iter.throw = function (value) {
        if (waiting) {
          waiting = false;
          throw value;
        }

        return pump("throw", value);
      };
    }

    if (typeof inner.return === "function") {
      iter.return = function (value) {
        return pump("return", value);
      };
    }

    return iter;
  };

  babelHelpers.asyncToGenerator = function (fn) {
    return function () {
      var gen = fn.apply(this, arguments);
      return new Promise(function (resolve, reject) {
        function step(key, arg) {
          try {
            var info = gen[key](arg);
            var value = info.value;
          } catch (error) {
            reject(error);
            return;
          }

          if (info.done) {
            resolve(value);
          } else {
            return Promise.resolve(value).then(function (value) {
              step("next", value);
            }, function (err) {
              step("throw", err);
            });
          }
        }

        return step("next");
      });
    };
  };

  babelHelpers.classCallCheck = function (instance, Constructor) {
    if (!(instance instanceof Constructor)) {
      throw new TypeError("Cannot call a class as a function");
    }
  };

  babelHelpers.createClass = function () {
    function defineProperties(target, props) {
      for (var i = 0; i < props.length; i++) {
        var descriptor = props[i];
        descriptor.enumerable = descriptor.enumerable || false;
        descriptor.configurable = true;
        if ("value" in descriptor) descriptor.writable = true;
        Object.defineProperty(target, descriptor.key, descriptor);
      }
    }

    return function (Constructor, protoProps, staticProps) {
      if (protoProps) defineProperties(Constructor.prototype, protoProps);
      if (staticProps) defineProperties(Constructor, staticProps);
      return Constructor;
    };
  }();

  babelHelpers.defineEnumerableProperties = function (obj, descs) {
    for (var key in descs) {
      var desc = descs[key];
      desc.configurable = desc.enumerable = true;
      if ("value" in desc) desc.writable = true;
      Object.defineProperty(obj, key, desc);
    }

    return obj;
  };

  babelHelpers.defaults = function (obj, defaults) {
    var keys = Object.getOwnPropertyNames(defaults);

    for (var i = 0; i < keys.length; i++) {
      var key = keys[i];
      var value = Object.getOwnPropertyDescriptor(defaults, key);

      if (value && value.configurable && obj[key] === undefined) {
        Object.defineProperty(obj, key, value);
      }
    }

    return obj;
  };

  babelHelpers.defineProperty = function (obj, key, value) {
    if (key in obj) {
      Object.defineProperty(obj, key, {
        value: value,
        enumerable: true,
        configurable: true,
        writable: true
      });
    } else {
      obj[key] = value;
    }

    return obj;
  };

  babelHelpers.extends = Object.assign || function (target) {
    for (var i = 1; i < arguments.length; i++) {
      var source = arguments[i];

      for (var key in source) {
        if (Object.prototype.hasOwnProperty.call(source, key)) {
          target[key] = source[key];
        }
      }
    }

    return target;
  };

  babelHelpers.get = function get(object, property, receiver) {
    if (object === null) object = Function.prototype;
    var desc = Object.getOwnPropertyDescriptor(object, property);

    if (desc === undefined) {
      var parent = Object.getPrototypeOf(object);

      if (parent === null) {
        return undefined;
      } else {
        return get(parent, property, receiver);
      }
    } else if ("value" in desc) {
      return desc.value;
    } else {
      var getter = desc.get;

      if (getter === undefined) {
        return undefined;
      }

      return getter.call(receiver);
    }
  };

  babelHelpers.inherits = function (subClass, superClass) {
    if (typeof superClass !== "function" && superClass !== null) {
      throw new TypeError("Super expression must either be null or a function, not " + typeof superClass);
    }

    subClass.prototype = Object.create(superClass && superClass.prototype, {
      constructor: {
        value: subClass,
        enumerable: false,
        writable: true,
        configurable: true
      }
    });
    if (superClass) Object.setPrototypeOf ? Object.setPrototypeOf(subClass, superClass) : subClass.__proto__ = superClass;
  };

  babelHelpers.instanceof = function (left, right) {
    if (right != null && typeof Symbol !== "undefined" && right[Symbol.hasInstance]) {
      return right[Symbol.hasInstance](left);
    } else {
      return left instanceof right;
    }
  };

  babelHelpers.interopRequireDefault = function (obj) {
    return obj && obj.__esModule ? obj : {
      default: obj
    };
  };

  babelHelpers.interopRequireWildcard = function (obj) {
    if (obj && obj.__esModule) {
      return obj;
    } else {
      var newObj = {};

      if (obj != null) {
        for (var key in obj) {
          if (Object.prototype.hasOwnProperty.call(obj, key)) newObj[key] = obj[key];
        }
      }

      newObj.default = obj;
      return newObj;
    }
  };

  babelHelpers.newArrowCheck = function (innerThis, boundThis) {
    if (innerThis !== boundThis) {
      throw new TypeError("Cannot instantiate an arrow function");
    }
  };

  babelHelpers.objectDestructuringEmpty = function (obj) {
    if (obj == null) throw new TypeError("Cannot destructure undefined");
  };

  babelHelpers.objectWithoutProperties = function (obj, keys) {
    var target = {};

    for (var i in obj) {
      if (keys.indexOf(i) >= 0) continue;
      if (!Object.prototype.hasOwnProperty.call(obj, i)) continue;
      target[i] = obj[i];
    }

    return target;
  };

  babelHelpers.possibleConstructorReturn = function (self, call) {
    if (!self) {
      throw new ReferenceError("this hasn't been initialised - super() hasn't been called");
    }

    return call && (typeof call === "object" || typeof call === "function") ? call : self;
  };

  babelHelpers.selfGlobal = typeof global === "undefined" ? self : global;

  babelHelpers.set = function set(object, property, value, receiver) {
    var desc = Object.getOwnPropertyDescriptor(object, property);

    if (desc === undefined) {
      var parent = Object.getPrototypeOf(object);

      if (parent !== null) {
        set(parent, property, value, receiver);
      }
    } else if ("value" in desc && desc.writable) {
      desc.value = value;
    } else {
      var setter = desc.set;

      if (setter !== undefined) {
        setter.call(receiver, value);
      }
    }

    return value;
  };

  babelHelpers.slicedToArray = function () {
    function sliceIterator(arr, i) {
      var _arr = [];
      var _n = true;
      var _d = false;
      var _e = undefined;

      try {
        for (var _i = arr[Symbol.iterator](), _s; !(_n = (_s = _i.next()).done); _n = true) {
          _arr.push(_s.value);

          if (i && _arr.length === i) break;
        }
      } catch (err) {
        _d = true;
        _e = err;
      } finally {
        try {
          if (!_n && _i["return"]) _i["return"]();
        } finally {
          if (_d) throw _e;
        }
      }

      return _arr;
    }

    return function (arr, i) {
      if (Array.isArray(arr)) {
        return arr;
      } else if (Symbol.iterator in Object(arr)) {
        return sliceIterator(arr, i);
      } else {
        throw new TypeError("Invalid attempt to destructure non-iterable instance");
      }
    };
  }();

  babelHelpers.slicedToArrayLoose = function (arr, i) {
    if (Array.isArray(arr)) {
      return arr;
    } else if (Symbol.iterator in Object(arr)) {
      var _arr = [];

      for (var _iterator = arr[Symbol.iterator](), _step; !(_step = _iterator.next()).done;) {
        _arr.push(_step.value);

        if (i && _arr.length === i) break;
      }

      return _arr;
    } else {
      throw new TypeError("Invalid attempt to destructure non-iterable instance");
    }
  };

  babelHelpers.taggedTemplateLiteral = function (strings, raw) {
    return Object.freeze(Object.defineProperties(strings, {
      raw: {
        value: Object.freeze(raw)
      }
    }));
  };

  babelHelpers.taggedTemplateLiteralLoose = function (strings, raw) {
    strings.raw = raw;
    return strings;
  };

  babelHelpers.temporalRef = function (val, name, undef) {
    if (val === undef) {
      throw new ReferenceError(name + " is not defined - temporal dead zone");
    } else {
      return val;
    }
  };

  babelHelpers.temporalUndefined = {};

  babelHelpers.toArray = function (arr) {
    return Array.isArray(arr) ? arr : Array.from(arr);
  };

  babelHelpers.toConsumableArray = function (arr) {
    if (Array.isArray(arr)) {
      for (var i = 0, arr2 = Array(arr.length); i < arr.length; i++) arr2[i] = arr[i];

      return arr2;
    } else {
      return Array.from(arr);
    }
  };
})(typeof global === "undefined" ? self : global);

/***/ }),
/* 15 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
Object.defineProperty(__webpack_exports__, "__esModule", { value: true });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__topologymanager_data_source_index__ = __webpack_require__(1);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__topologymanager_topology_layout_index__ = __webpack_require__(19);


window.TopologyORegistry = {
    dataSources: {
        infraTopology: __WEBPACK_IMPORTED_MODULE_0__topologymanager_data_source_index__["c" /* InfraTopologyDataSource */],
        hostTopology: __WEBPACK_IMPORTED_MODULE_0__topologymanager_data_source_index__["b" /* HostTopologyDataSource */]
    },
    layouts: {
        skydive_default: __WEBPACK_IMPORTED_MODULE_1__topologymanager_topology_layout_index__["b" /* SkydiveDefaultLayout */],
        infra: __WEBPACK_IMPORTED_MODULE_1__topologymanager_topology_layout_index__["c" /* SkydiveInfraLayout */]
    },
    config: __WEBPACK_IMPORTED_MODULE_1__topologymanager_topology_layout_index__["a" /* LayoutConfig */]
};


/***/ }),
/* 16 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class DataSourceRegistry {
    constructor() {
        this.sources = [];
    }
    addSource(source, defaultSource) {
        this.sources.push(source);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = DataSourceRegistry;



/***/ }),
/* 17 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_events__);

class InfraTopologyDataSource {
    constructor() {
        this.sourceType = "skydive";
        this.dataSourceName = "infra_topology";
        this.e = new __WEBPACK_IMPORTED_MODULE_0_events__["EventEmitter"]();
        this.subscribable = true;
        this.filterQuery = "G.V().Has('Type', 'host')";
        this.onConnected = this.onConnected.bind(this);
        this.processMessage = this.processMessage.bind(this);
    }
    subscribe() {
        window.websocket.disconnect();
        window.websocket.removeMsgHandler('Graph', this.processMessage);
        window.websocket.addMsgHandler('Graph', this.processMessage);
        window.websocket.addConnectHandler(this.onConnected, true);
    }
    unsubscribe() {
        this.e.removeAllListeners();
        window.websocket.removeMsgHandler('Graph', this.processMessage);
        window.websocket.disconnect();
    }
    onConnected() {
        console.log('Send sync request');
        const obj = {};
        if (this.time) {
            obj.Time = this.time;
        }
        obj.GremlinFilter = this.filterQuery + ".SubGraph()";
        const msg = { "Namespace": "Graph", "Type": "SyncRequest", "Obj": obj };
        window.websocket.send(msg);
    }
    processMessage(msg) {
        console.log('Got message from websocket', msg);
        this.e.emit('broadcastMessage', msg.Type, msg);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = InfraTopologyDataSource;



/***/ }),
/* 18 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0_events___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_0_events__);

class HostTopologyDataSource {
    constructor(host) {
        this.sourceType = "skydive";
        this.dataSourceName = "infra_topology";
        this.e = new __WEBPACK_IMPORTED_MODULE_0_events__["EventEmitter"]();
        this.subscribable = true;
        this.filterQuery = "";
        this.filterQuery = "G.V().Has('Host', '" + host + "')";
        this.onConnected = this.onConnected.bind(this);
        this.processMessage = this.processMessage.bind(this);
    }
    subscribe() {
        window.websocket.removeMsgHandler('Graph', this.processMessage);
        window.websocket.addMsgHandler('Graph', this.processMessage);
        window.websocket.addConnectHandler(this.onConnected, true);
    }
    unsubscribe() {
        this.e.removeAllListeners();
        window.websocket.removeMsgHandler('Graph', this.processMessage);
    }
    onConnected() {
        console.log('Send sync request');
        const obj = {};
        if (this.time) {
            obj.Time = this.time;
        }
        obj.GremlinFilter = this.filterQuery + ".SubGraph()";
        const msg = { "Namespace": "Graph", "Type": "SyncRequest", "Obj": obj };
        console.log('send msg', msg);
        window.websocket.send(msg);
    }
    processMessage(msg) {
        console.log('Got message from websocket', msg);
        this.e.emit('broadcastMessage', msg.Type, msg);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = HostTopologyDataSource;



/***/ }),
/* 19 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__config__ = __webpack_require__(20);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__config__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__skydive_default_index__ = __webpack_require__(25);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return __WEBPACK_IMPORTED_MODULE_1__skydive_default_index__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__infra_index__ = __webpack_require__(34);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return __WEBPACK_IMPORTED_MODULE_2__infra_index__["a"]; });





/***/ }),
/* 20 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
const get = __webpack_require__(21);
const set = __webpack_require__(23);
class LayoutConfig {
    constructor(configuration) {
        this.configuration = configuration;
    }
    getValue(pathInConfig, ...args) {
        const val = get(this.configuration, pathInConfig);
        if (typeof val === 'function') {
            return val(...args);
        }
        return val;
    }
    setValue(pathInConfig, val) {
        set(this.configuration, pathInConfig, val);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutConfig;



/***/ }),
/* 21 */
/***/ (function(module, exports, __webpack_require__) {

/*!
 * get-value <https://github.com/jonschlinkert/get-value>
 *
 * Copyright (c) 2014-2018, Jon Schlinkert.
 * Released under the MIT License.
 */

const isObject = __webpack_require__(2);

module.exports = function(target, path, options) {
  if (!isObject(options)) {
    options = { default: options };
  }

  if (!isValidObject(target)) {
    return typeof options.default !== 'undefined' ? options.default : target;
  }

  if (typeof path === 'number') {
    path = String(path);
  }

  const isArray = Array.isArray(path);
  const isString = typeof path === 'string';
  const splitChar = options.separator || '.';
  const joinChar = options.joinChar || (typeof splitChar === 'string' ? splitChar : '.');

  if (!isString && !isArray) {
    return target;
  }

  if (isString && path in target) {
    return isValid(path, target, options) ? target[path] : options.default;
  }

  let segs = isArray ? path : split(path, splitChar, options);
  let len = segs.length;
  let idx = 0;

  do {
    let prop = segs[idx];
    if (typeof prop === 'number') {
      prop = String(prop);
    }

    while (prop && prop.slice(-1) === '\\') {
      prop = join([prop.slice(0, -1), segs[++idx] || ''], joinChar, options);
    }

    if (prop in target) {
      if (!isValid(prop, target, options)) {
        return options.default;
      }

      target = target[prop];
    } else {
      let hasProp = false;
      let n = idx + 1;

      while (n < len) {
        prop = join([prop, segs[n++]], joinChar, options);

        if ((hasProp = prop in target)) {
          if (!isValid(prop, target, options)) {
            return options.default;
          }

          target = target[prop];
          idx = n - 1;
          break;
        }
      }

      if (!hasProp) {
        return options.default;
      }
    }
  } while (++idx < len && isValidObject(target));

  if (idx === len) {
    return target;
  }

  return options.default;
};

function join(segs, joinChar, options) {
  if (typeof options.join === 'function') {
    return options.join(segs);
  }
  return segs[0] + joinChar + segs[1];
}

function split(path, splitChar, options) {
  if (typeof options.split === 'function') {
    return options.split(path);
  }
  return path.split(splitChar);
}

function isValid(key, target, options) {
  if (typeof options.isValid === 'function') {
    return options.isValid(key, target);
  }
  return true;
}

function isValidObject(val) {
  return isObject(val) || Array.isArray(val) || typeof val === 'function';
}


/***/ }),
/* 22 */
/***/ (function(module, exports) {

var toString = {}.toString;

module.exports = Array.isArray || function (arr) {
  return toString.call(arr) == '[object Array]';
};


/***/ }),
/* 23 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * set-value <https://github.com/jonschlinkert/set-value>
 *
 * Copyright (c) 2014-2018, Jon Schlinkert.
 * Released under the MIT License.
 */



const isPlain = __webpack_require__(24);

function set(target, path, value, options) {
  if (!isObject(target)) {
    return target;
  }

  let opts = options || {};
  const isArray = Array.isArray(path);
  if (!isArray && typeof path !== 'string') {
    return target;
  }

  let merge = opts.merge;
  if (merge && typeof merge !== 'function') {
    merge = Object.assign;
  }

  const keys = isArray ? path : split(path, opts);
  const len = keys.length;
  const orig = target;

  if (!options && keys.length === 1) {
    result(target, keys[0], value, merge);
    return target;
  }

  for (let i = 0; i < len; i++) {
    let prop = keys[i];

    if (!isObject(target[prop])) {
      target[prop] = {};
    }

    if (i === len - 1) {
      result(target, prop, value, merge);
      break;
    }

    target = target[prop];
  }

  return orig;
}

function result(target, path, value, merge) {
  if (merge && isPlain(target[path]) && isPlain(value)) {
    target[path] = merge({}, target[path], value);
  } else {
    target[path] = value;
  }
}

function split(path, options) {
  const id = createKey(path, options);
  if (set.memo[id]) return set.memo[id];

  const char = (options && options.separator) ? options.separator : '.';
  let keys = [];
  let res = [];

  if (options && typeof options.split === 'function') {
    keys = options.split(path);
  } else {
    keys = path.split(char);
  }

  for (let i = 0; i < keys.length; i++) {
    let prop = keys[i];
    while (prop && prop.slice(-1) === '\\' && keys[i + 1]) {
      prop = prop.slice(0, -1) + char + keys[++i];
    }
    res.push(prop);
  }
  set.memo[id] = res;
  return res;
}

function createKey(pattern, options) {
  let id = pattern;
  if (typeof options === 'undefined') {
    return id + '';
  }
  const keys = Object.keys(options);
  for (let i = 0; i < keys.length; i++) {
    const key = keys[i];
    id += ';' + key + '=' + String(options[key]);
  }
  return id;
}

function isObject(val) {
  switch (typeof val) {
    case 'null':
      return false;
    case 'object':
      return true;
    case 'function':
      return true;
    default: {
      return false;
    }
  }
}

set.memo = {};
module.exports = set;


/***/ }),
/* 24 */
/***/ (function(module, exports, __webpack_require__) {

"use strict";
/*!
 * is-plain-object <https://github.com/jonschlinkert/is-plain-object>
 *
 * Copyright (c) 2014-2017, Jon Schlinkert.
 * Released under the MIT License.
 */



var isObject = __webpack_require__(2);

function isObjectObject(o) {
  return isObject(o) === true
    && Object.prototype.toString.call(o) === '[object Object]';
}

module.exports = function isPlainObject(o) {
  var ctor,prot;

  if (isObjectObject(o) === false) return false;

  // If has modified constructor
  ctor = o.constructor;
  if (typeof ctor !== 'function') return false;

  // If has modified prototype
  prot = ctor.prototype;
  if (isObjectObject(prot) === false) return false;

  // If constructor does not have an Object-specific method
  if (prot.hasOwnProperty('isPrototypeOf') === false) {
    return false;
  }

  // Most likely a plain Object
  return true;
};


/***/ }),
/* 25 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__data_source_index__ = __webpack_require__(1);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_events__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_events___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_events__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__base_index__ = __webpack_require__(3);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__base_ui_index__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__base_edge_label_index__ = __webpack_require__(12);





class SkydiveDefaultLayout {
    constructor(selector) {
        this.dataManager = new __WEBPACK_IMPORTED_MODULE_2__base_index__["a" /* DataManager */]();
        this.e = new __WEBPACK_IMPORTED_MODULE_1_events__["EventEmitter"]();
        this.alias = "skydive_default";
        this.active = false;
        this.dataSources = new __WEBPACK_IMPORTED_MODULE_0__data_source_index__["a" /* DataSourceRegistry */]();
        this.selector = selector;
        this.uiBridge = new __WEBPACK_IMPORTED_MODULE_3__base_ui_index__["a" /* LayoutBridgeUI */](selector);
        this.uiBridge.useEventEmitter(this.e);
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useDataManager(this.dataManager);
        this.uiBridge.setCollapseLevel(1);
        this.uiBridge.setMinimumCollapseLevel(1);
        this.dataManager.useLayoutContext(this.uiBridge.layoutContext);
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        $(this.selector).empty();
        this.active = true;
    }
    useLinkLabelStrategy(linkLabelType) {
        const strategy = Object(__WEBPACK_IMPORTED_MODULE_4__base_edge_label_index__["a" /* LabelRetrieveInformationStrategy */])(linkLabelType);
        strategy.setup(this.config);
        this.uiBridge.useLinkLabelStrategy(strategy);
    }
    useConfig(config) {
        this.config = config;
        this.uiBridge.useConfig(this.config);
    }
    remove() {
        this.dataSources.sources.forEach((source) => {
            source.unsubscribe();
        });
        this.active = false;
        this.uiBridge.remove();
        $(this.selector).empty();
    }
    addDataSource(dataSource, defaultSource) {
        this.dataSources.addSource(dataSource, !!defaultSource);
    }
    reactToDataSourceEvent(dataSource, eventName, ...args) {
        console.log('Skydive default layout got an event', eventName, args);
        switch (eventName) {
            case "SyncReply":
                if (this.config.getValue('useHardcodedData')) {
                    this.dataManager.updateFromData(dataSource.sourceType, window.detailedTopology);
                }
                else {
                    this.dataManager.updateFromData(dataSource.sourceType, args[0]);
                }
                console.log('Built dataManager', this.dataManager);
                $(this.selector).empty();
                this.uiBridge.useDataManager(this.dataManager);
                this.uiBridge.start();
                this.e.emit('ui.update');
                break;
            // case "NodeAdded":
            //     this.dataManager.addNodeFromData(dataSource.sourceType, args[0]);
            //     console.log('Added node', args[0]);
            //     this.e.emit('ui.update');
            //     break;
            // case "NodeDeleted":
            //     this.dataManager.removeNodeFromData(dataSource.sourceType, args[0]);
            //     console.log('Deleted node', args[0]);
            //     this.e.emit('ui.update');
            //     break;
            // case "NodeUpdated":
            //     const nodeOldAndNew = this.dataManager.updateNodeFromData(dataSource.sourceType, args[0]);
            //     console.log('Updated node', args[0]);
            //     this.e.emit('node.updated', nodeOldAndNew.oldNode, nodeOldAndNew.newNode);
            //     break;
            // case "HostGraphDeleted":
            //     this.dataManager.removeAllNodesWhichBelongsToHostFromData(dataSource.sourceType, args[0]);
            //     console.log('Removed host', args[0]);
            //     this.e.emit('ui.updated');
            //     break;
        }
    }
    reactToTheUiEvent(eventName, ...args) {
        this.e.emit('ui.' + eventName, ...args);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = SkydiveDefaultLayout;



/***/ }),
/* 26 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__node_index__ = __webpack_require__(4);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__edge_index__ = __webpack_require__(6);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__group_index__ = __webpack_require__(29);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__parsers_index__ = __webpack_require__(30);




class DataManager {
    constructor() {
        this.nodeManager = new __WEBPACK_IMPORTED_MODULE_0__node_index__["a" /* NodeRegistry */]();
        this.edgeManager = new __WEBPACK_IMPORTED_MODULE_1__edge_index__["a" /* EdgeRegistry */]();
        this.groupManager = new __WEBPACK_IMPORTED_MODULE_2__group_index__["a" /* GroupRegistry */]();
    }
    useLayoutContext(layoutContext) {
        this.layoutContext = layoutContext;
    }
    addNodeFromData(dataType, data) {
        Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["d" /* parseSkydiveMessageWithOneNode */])(this, data);
    }
    removeNodeFromData(dataType, data) {
        const nodeID = Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["c" /* getNodeIDFromSkydiveMessageWithOneNode */])(data);
        this.nodeManager.removeNodeByID(nodeID);
    }
    updateNodeFromData(dataType, data) {
        const nodeID = Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["c" /* getNodeIDFromSkydiveMessageWithOneNode */])(data);
        const node = this.nodeManager.getNodeById(nodeID);
        const clonedOldNode = node.clone();
        Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["e" /* parseSkydiveMessageWithOneNodeAndUpdateNode */])(node, data);
        return { oldNode: clonedOldNode, newNode: node };
    }
    removeAllNodesWhichBelongsToHostFromData(dataType, data) {
        const nodeHost = Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["b" /* getHostFromSkydiveMessageWithOneNode */])(data);
        this.nodeManager.removeNodeByHost(nodeHost);
    }
    updateFromData(dataType, data) {
        Object(__WEBPACK_IMPORTED_MODULE_3__parsers_index__["a" /* default */])(this, dataType, data);
    }
    removeOldData() {
        this.nodeManager.removeOldData();
        this.edgeManager.removeOldData();
        this.groupManager.removeOldData();
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = DataManager;



/***/ }),
/* 27 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__node__ = __webpack_require__(5);

class NodeRegistry {
    constructor() {
        this.nodes = [];
    }
    addNodeFromData(ID, Name, Host, Metadata) {
        this.nodes.push(__WEBPACK_IMPORTED_MODULE_0__node__["a" /* default */].createFromData(ID, Name, Host, Metadata));
    }
    getActive() {
        return this.nodes.find((n) => n.selected);
    }
    getNodeById(ID) {
        return this.nodes.find((n) => n.ID === ID);
    }
    get size() {
        return this.nodes.length;
    }
    removeNodeByID(nodeID) {
        this.nodes = this.nodes.filter((n) => n.ID !== nodeID);
    }
    removeNodeByHost(nodeHost) {
        this.nodes = this.nodes.filter((n) => n.Host !== nodeHost);
    }
    isThereAnyNodeWithType(Type) {
        return !!this.nodes.some((n) => n.Metadata.Type === Type);
    }
    addNode(node) {
        this.nodes.push(node);
    }
    getVisibleNodes(visibilityLevel = 1, autoExpand = false) {
        const nodes = this.nodes.filter((node) => {
            // if (node.Name === 'tapbbbf73d3-6a') {
            //     console.log(visibilityLevel, autoExpand, node.group, node.isGroupOwner(), node.visible);
            // }
            if (autoExpand) {
                return true;
            }
            if (node.group && node.group.level > visibilityLevel) {
                if (node.isGroupOwner() && node.group.level === visibilityLevel + 1) {
                    return true;
                }
                if (!node.group.collapsed) {
                    return true;
                }
                return node.visible;
            }
            if (node.isGroupOwner()) {
                return true;
            }
            if (node.group && !node.group.collapsed) {
                return true;
            }
            if (!node.group) {
                return true;
            }
            if (!node.group.collapsed) {
                return true;
            }
            return node.visible;
        });
        nodes.forEach((n) => n.visible = true);
        return nodes;
    }
    removeOldData() {
        this.nodes = [];
    }
    removeEdgeByID(ID) {
        this.nodes.forEach((n) => {
            n.edges.removeEdgeByID(ID);
        });
    }
    groupRemoved(g) {
        this.nodes.forEach((n) => {
            if (n.group && n.group.isEqualTo(g)) {
                n.group = null;
            }
        });
    }
    clone() {
        const registry = new NodeRegistry();
        this.nodes.forEach((n) => registry.addNode(n));
        return registry;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = NodeRegistry;



/***/ }),
/* 28 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__edge__ = __webpack_require__(7);

class EdgeRegistry {
    constructor() {
        this.edges = [];
    }
    addEdgeFromData(ID, Host, Metadata, source, target) {
        this.edges.push(__WEBPACK_IMPORTED_MODULE_0__edge__["a" /* default */].createFromData(ID, Host, Metadata, source, target));
    }
    getEdgeById(ID) {
        return this.edges.find((e) => e.ID === ID);
    }
    get size() {
        return this.edges.length;
    }
    removeEdgeByID(ID) {
        this.edges = this.edges.filter((e) => e.ID !== ID);
    }
    removeEdgeByHost(host) {
        this.edges = this.edges.filter((e) => e.Host !== host);
    }
    getEdgesWithRelationType(relationType) {
        return this.edges.filter((e) => e.hasRelationType(relationType));
    }
    addEdge(e) {
        this.edges.push(e);
    }
    removeOldData() {
        this.edges = [];
    }
    getVisibleEdges(visibleNodes) {
        const visibleNodeIds = visibleNodes.reduce((accum, node) => {
            accum.push(node.ID);
            return accum;
        }, []);
        return this.edges.filter((e) => {
            return visibleNodeIds.indexOf(e.source.ID) !== -1 && visibleNodeIds.indexOf(e.target.ID) !== -1;
        });
    }
    getActive() {
        return this.edges.find((n) => n.selected);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = EdgeRegistry;



/***/ }),
/* 29 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__registry__ = __webpack_require__(8);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "a", function() { return __WEBPACK_IMPORTED_MODULE_0__registry__["a"]; });
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1__group__ = __webpack_require__(9);
/* unused harmony reexport Group */




/***/ }),
/* 30 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["a"] = parseData;
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__skydive__ = __webpack_require__(10);
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "d", function() { return __WEBPACK_IMPORTED_MODULE_0__skydive__["d"]; });
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "c", function() { return __WEBPACK_IMPORTED_MODULE_0__skydive__["c"]; });
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "e", function() { return __WEBPACK_IMPORTED_MODULE_0__skydive__["e"]; });
/* harmony reexport (binding) */ __webpack_require__.d(__webpack_exports__, "b", function() { return __WEBPACK_IMPORTED_MODULE_0__skydive__["b"]; });


function parseData(dataManager, dataType, data) {
    const parsers = {
        skydive: __WEBPACK_IMPORTED_MODULE_0__skydive__["a" /* default */]
    };
    if (!parsers[dataType]) {
        throw new Error("No registered parser for dataType " + dataType);
    }
    return parsers[dataType](dataManager, data);
}


/***/ }),
/* 31 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__layout_context__ = __webpack_require__(32);

class LayoutBridgeUI {
    constructor(selector) {
        this.initialized = false;
        this.collapseLevel = 1;
        this.minimumCollapseLevel = 1;
        this.autoExpand = false;
        this.selector = selector;
    }
    useEventEmitter(e) {
        this.e = e;
    }
    useLinkLabelStrategy(linkLabelStrategy) {
        this.linkLabelStrategy = linkLabelStrategy;
    }
    setAutoExpand(autoExpand) {
        this.autoExpand = autoExpand;
    }
    setCollapseLevel(level) {
        this.collapseLevel = level;
    }
    setMinimumCollapseLevel(level) {
        this.minimumCollapseLevel = level;
    }
    useDataManager(dataManager) {
        this.dataManager = dataManager;
    }
    useConfig(config) {
        this.config = config;
    }
    start() {
        this.initialized = false;
        this.initialized = true;
    }
    remove() {
    }
    get layoutContext() {
        const context = new __WEBPACK_IMPORTED_MODULE_0__layout_context__["a" /* default */]();
        context.getCollapseLevel = () => this.collapseLevel;
        context.getMinimumCollapseLevel = () => this.minimumCollapseLevel;
        context.isAutoExpand = () => this.autoExpand;
        context.dataManager = this.dataManager;
        context.e = this.e;
        context.config = this.config;
        context.linkLabelStrategy = this.linkLabelStrategy;
        return context;
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutBridgeUI;



/***/ }),
/* 32 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
class LayoutContext {
    getCollapseLevel() {
        return 1;
    }
    getMinimumCollapseLevel() {
        return 1;
    }
    isAutoExpand() {
        return false;
    }
    get collapseLevel() {
        return Math.max(this.getCollapseLevel(), this.getMinimumCollapseLevel());
    }
    subscribeToEvent(eventName, cb) {
        this.e.on(eventName, cb);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = LayoutContext;



/***/ }),
/* 33 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony export (immutable) */ __webpack_exports__["a"] = getLinkLabelRetrieveInformationStrategy;
function getLinkLabelRetrieveInformationStrategy(linkLabelType) {
    const typeToStrategy = {
        "latency": LatencyStrategy,
        "bandwidth": BandwidthStrategy
    };
    return new typeToStrategy[linkLabelType]();
}
const maxClockSkewMillis = 5 * 60 * 1000; // 5 minutes
function bandwidthToString(bps) {
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
    constructor() {
        this.client = window.apiMixin;
        this.client.created();
    }
    setup(config) {
        this.config = config;
        this.active = 0;
        this.warning = 10;
        this.alert = 100;
    }
    updateLatency(edge, a, b) {
        edge.latencyTimestamp = Math.max(a.Last, b.Last);
        edge.latency = Math.abs(a.RTT - b.RTT) / 1000000;
    }
    flowQuery(nodeTID, trackingID, limit) {
        let has = `"NodeTID", ${nodeTID}`;
        if (typeof trackingID !== 'undefined') {
            has += `"TrackingID", ${trackingID}`;
        }
        has += `"RTT", NE(0)`;
        let query = `G.Flows().Has(${has}).Sort().Limit(${limit})`;
        return this.client.$topologyQuery(query);
    }
    flowQueryByNodeTID(nodeTID, limit) {
        return this.flowQuery(`"${nodeTID}"`, undefined, limit);
    }
    flowQueryByNodeTIDandTrackingID(nodeTID, flows) {
        let anyTrackingID = 'Within(';
        let i;
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
    flowCategoryKey(flow) {
        return `a=${flow.Link.A} b=${flow.Link.B} app=${flow.Application}`;
    }
    uniqueFlows(inFlows, count) {
        let outFlows = [];
        let hasCategory = {};
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
    mapFlowByTrackingID(flows) {
        let map = {};
        for (let i in flows) {
            const flow = flows[i];
            map[flow.TrackingID] = flow;
        }
        return map;
    }
    updateData(edge) {
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
            .then((aFlows) => {
            if (aFlows.length === 0) {
                return;
            }
            const maxUniqueFlows = 100;
            aFlows = this.uniqueFlows(aFlows, maxUniqueFlows);
            const aFlowMap = this.mapFlowByTrackingID(aFlows);
            this.flowQueryByNodeTIDandTrackingID(b.TID, aFlows)
                .then((bFlows) => {
                if (bFlows.length === 0) {
                    return;
                }
                const bFlow = bFlows[0];
                const aFlow = aFlowMap[bFlow.TrackingID];
                this.updateLatency(edge, aFlow, bFlow);
            })
                .catch(function (error) {
                console.log(error);
            });
        })
            .catch(function (error) {
            console.log(error);
        });
    }
    hasData(edge) {
        if (!edge.latencyTimestamp) {
            return false;
        }
        const elapsedMillis = Date.now() - (+(new Date(edge.latencyTimestamp)));
        return elapsedMillis <= maxClockSkewMillis;
    }
    getText(edge) {
        return `${edge.latency} ms`;
    }
    isActive(edge) {
        return (edge.latency >= this.active) && (edge.latency < this.warning);
    }
    isWarning(edge) {
        return (edge.latency >= this.warning) && (edge.latency < this.alert);
    }
    isAlert(edge) {
        return (edge.latency >= this.alert);
    }
}
class BandwidthStrategy {
    constructor() {
        this.client = window.apiMixin;
        this.client.created();
    }
    bandwidthFromMetrics(metrics) {
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
    setup(config) {
        this.config = config;
    }
    updateData(edge) {
        var metadata;
        if (edge.target.Metadata.LastUpdateMetric) {
            metadata = edge.target.Metadata;
        }
        else if (edge.source.Metadata.LastUpdateMetric) {
            metadata = edge.source.Metadata;
        }
        else {
            return;
        }
        const defaultBandwidthBaseline = 1024 * 1024 * 1024; // 1 gbps
        edge.bandwidthBaseline = (this.config.getValue('bandwidth').bandwidthThreshold === 'relative') ?
            metadata.Speed || defaultBandwidthBaseline : 1;
        edge.bandwidthAbsolute = this.bandwidthFromMetrics(metadata.LastUpdateMetric);
        edge.bandwidth = edge.bandwidthAbsolute / edge.bandwidthBaseline;
    }
    hasData(edge) {
        if (!edge.target.Metadata.LastUpdateMetric && !edge.source.Metadata.LastUpdateMetric) {
            return false;
        }
        if (!edge.bandwidth) {
            return false;
        }
        return edge.bandwidth > this.config.getValue('bandwidth').active;
    }
    getText(edge) {
        return bandwidthToString(edge.bandwidthAbsolute);
    }
    isActive(edge) {
        return (edge.bandwidth > this.config.getValue('bandwidth').active) && (edge.bandwidth < this.config.getValue('bandwidth').warning);
    }
    isWarning(edge) {
        return (edge.bandwidth >= this.config.getValue('bandwidth').warning) && (edge.bandwidth < this.config.getValue('bandwidth').alert);
    }
    isAlert(edge) {
        return edge.bandwidth >= this.config.getValue('bandwidth').alert;
    }
}


/***/ }),
/* 34 */
/***/ (function(module, __webpack_exports__, __webpack_require__) {

"use strict";
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_0__data_source_index__ = __webpack_require__(1);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_events__ = __webpack_require__(0);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_1_events___default = __webpack_require__.n(__WEBPACK_IMPORTED_MODULE_1_events__);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_2__base_index__ = __webpack_require__(3);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_3__base_ui_index__ = __webpack_require__(11);
/* harmony import */ var __WEBPACK_IMPORTED_MODULE_4__base_edge_label_index__ = __webpack_require__(12);





class SkydiveInfraLayout {
    constructor(selector) {
        this.dataManager = new __WEBPACK_IMPORTED_MODULE_2__base_index__["a" /* DataManager */]();
        this.e = new __WEBPACK_IMPORTED_MODULE_1_events__["EventEmitter"]();
        this.alias = "skydive_infra";
        this.active = false;
        this.dataSources = new __WEBPACK_IMPORTED_MODULE_0__data_source_index__["a" /* DataSourceRegistry */]();
        this.selector = selector;
        this.uiBridge = new __WEBPACK_IMPORTED_MODULE_3__base_ui_index__["a" /* LayoutBridgeUI */](selector);
        this.uiBridge.useEventEmitter(this.e);
        this.uiBridge.useConfig(this.config);
        this.uiBridge.useDataManager(this.dataManager);
        this.dataManager.useLayoutContext(this.uiBridge.layoutContext);
    }
    initializer() {
        console.log("Try to initialize topology " + this.alias);
        $(this.selector).empty();
        this.active = true;
        this.uiBridge.start();
    }
    useLinkLabelStrategy(linkLabelType) {
        const strategy = Object(__WEBPACK_IMPORTED_MODULE_4__base_edge_label_index__["a" /* LabelRetrieveInformationStrategy */])(linkLabelType);
        strategy.setup(this.config);
        this.uiBridge.useLinkLabelStrategy(strategy);
    }
    useConfig(config) {
        this.config = config;
        this.uiBridge.useConfig(this.config);
    }
    remove() {
        this.dataSources.sources.forEach((source) => {
            source.unsubscribe();
        });
        this.active = false;
        this.uiBridge.remove();
        $(this.selector).empty();
    }
    addDataSource(dataSource, defaultSource) {
        this.dataSources.addSource(dataSource, !!defaultSource);
    }
    reactToDataSourceEvent(dataSource, eventName, ...args) {
        console.log('Infra layout got an event', eventName, args);
        switch (eventName) {
            case "SyncReply":
                this.dataManager.updateFromData(dataSource.sourceType, args[0]);
                console.log('Built dataManager', this.dataManager);
                $(this.selector).empty();
                this.uiBridge.useDataManager(this.dataManager);
                this.e.emit('ui.update');
                break;
            case "NodeAdded":
                this.dataManager.addNodeFromData(dataSource.sourceType, args[0]);
                console.log('Added node', args[0]);
                this.e.emit('ui.update');
                break;
            case "NodeDeleted":
                this.dataManager.removeNodeFromData(dataSource.sourceType, args[0]);
                console.log('Deleted node', args[0]);
                this.e.emit('ui.update');
                break;
            case "NodeUpdated":
                const nodeOldAndNew = this.dataManager.updateNodeFromData(dataSource.sourceType, args[0]);
                console.log('Updated node', args[0]);
                this.e.emit('node.updated', nodeOldAndNew.oldNode, nodeOldAndNew.newNode);
                break;
            case "HostGraphDeleted":
                this.dataManager.removeAllNodesWhichBelongsToHostFromData(dataSource.sourceType, args[0]);
                console.log('Removed host', args[0]);
                this.e.emit('ui.updated');
                break;
        }
    }
    reactToTheUiEvent(eventName, ...args) {
        this.e.emit('ui.' + eventName, ...args);
    }
}
/* harmony export (immutable) */ __webpack_exports__["a"] = SkydiveInfraLayout;



/***/ })
/******/ ]);