function Expression() {
}
Expression.prototype.isNodeSelected = function (node) {
    return true;
}

function EQExpression(metadata, val) {
    Expression.call(this);
    this.metadata = metadata;
    this.value = val;
}
EQExpression.prototype = Object.create(Expression.prototype);
EQExpression.prototype.isNodeSelected = function(node) {
    return node.metadata[this.metadata] == this.value;
}

function NotEQExpression(metadata, val) {
    Expression.call(this);
    this.metadata = metadata;
    this.value = val;
}
NotEQExpression.prototype = Object.create(Expression.prototype);
NotEQExpression.prototype.isNodeSelected = function(node) {
    return node.metadata[this.metadata] != this.value;
}

function NotExpression(expr) {
    Expression.call(this);
    this.expr1 = expr;
}
NotExpression.prototype = Object.create(Expression.prototype);
NotExpression.prototype.isNodeSelected = function(node) {
    return !(this.expr1.isNodeSelected(node));
}

function BinaryExpression(expr1, expr2) {
    Expression.call(this);
    this.expr1 = expr1;
    this.expr2 = expr2;
}
BinaryExpression.prototype = Object.create(Expression.prototype);
BinaryExpression.prototype.isNodeSelected = function(node){return false;}

function OrBinaryExpression(expr1, expr2) {
    BinaryExpression.call(this, expr1, expr2);
}
OrBinaryExpression.prototype = Object.create(Expression.prototype);
OrBinaryExpression.prototype.isNodeSelected = function(node) {
    return this.expr1.isNodeSelected(node) || this.expr2.isNodeSelected(node);
}

function AndBinaryExpression(expr1, expr2) {
    BinaryExpression.call(this, expr1, expr2);
}
AndBinaryExpression.prototype = Object.create(Expression.prototype);
AndBinaryExpression.prototype.isNodeSelected = function(node) {
    return this.expr1.isNodeSelected(node) && this.expr2.isNodeSelected(node);
}

function parseExpr(str) {
    str = str.trim(" ")
    var leftBrCount = 0;
    var index = 0;
    if (str.length < 1) {
        return new Expression();
    }

    if (str[index] == '(') {
        leftBrCount = 1;
        index++
    } else if (str[index] == '!') {
        return new NotExpression(parseExpr(str.slice(1)));
    } else {
        var op = str.indexOf("=");
        if (op > 0 ) {
            if (str[op-1] == "!") {
                return new NotEQExpression(str.slice(0, op-1).trim(" "), str.slice(op+1).trim(" "))
            } else {
                return new EQExpression(str.split('=')[0].trim(" "), str.split('=')[1].trim(" "))
            }
        } else return new Expression();
    }

    // first character was '(' - now we find our
    // if its closure is in the middle of the expression
    while((leftBrCount != 0 ) && (index < str.length)) {
        if (str[index] == '(') {
            leftBrCount++;
        } else if (str[index] == ')'){
            leftBrCount--;
        }
            index++;
    }

    if (index < str.length) {
        trimmedStr = str.slice(index).trim(" ")
        if (trimmedStr.startsWith("&&")) {
            return new AndBinaryExpression(parseExpr(str.slice(0, index)), parseExpr(trimmedStr.slice(2)));
        }
        if (trimmedStr.startsWith("||")) {
            return new OrBinaryExpression(parseExpr(str.slice(0, index)), parseExpr(trimmedStr.slice(2)));
        }
    }
    return parseExpr(str.slice(1, str.length-1))
}

// X=500, Y=600, color=blue
function parseAction(str) {
    str = str.replace( /[()]/g, '' );
    actions=str.split(",");
    var myActions = {};
    for (var i=0; i<actions.length; i++) {
        myActions[i] = [actions[i].split("=")[0].trim(" "), parseInt(actions[i].split("=")[1].trim(" "))];
    }
    return myActions;
}

function ConditionActionRule(expr, actions){
    this.expr = expr
    this.actions = actions
}
ConditionActionRule.prototype.RePaintNode = function (node, layout) {
  var theNode = layout.nodes[node.id];
  if (theNode == null) return

  var hasFixedLocation = false;
  for(act in this.actions) {
     var command = {};
     var i = 0;
     for (elem in this.actions[act]){
        command[i] = this.actions[act][elem];
        i++;
     }
     switch(command[0]) {
     case "X": {
        theNode.x = command[1];
        theNode.fx = node.x;
        hasFixedLocation = true;
     }
     case "Y":{
        theNode.y = command[1];
        theNode.fy = node.y;
        hasFixedLocation = true;
     }
     default:
     }
  }
  if (hasFixedLocation) {
     layout.pinNode(theNode);
  }
}

function FixedPositionBuilder() {
   this.cond_rules = []
}
FixedPositionBuilder.prototype.RePaintNode = function (node, layout) {
        if (this.cond_rules == null) {
            return;
        }
        for(var i=0; i< this.cond_rules.length; i++){
           if (this.cond_rules[i].expr.isNodeSelected(node))
           {
                return this.cond_rules[i].RePaintNode(node, layout);
           }
        }
        return;
}
FixedPositionBuilder.prototype.AddRule = function (condition, action) {
    var exp = parseExpr(condition);
    var actions = parseAction(action);
    this.cond_rules.push(new ConditionActionRule(exp, actions));
}

