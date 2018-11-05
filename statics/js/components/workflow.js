Vue.component('item', {
  name: 'workflow-item',

  props: [
    'Name',
    'Description',
    'Type',
    'Values',
    'Default'
  ],

  inject: [
    'formData'
  ],

  template: `
  <div class="form-group">
    <div v-if="Type == 'string'" class="form-group">
      <label :for="Name">{{Description}}</label>
      <textarea:id="Name" v-model="formData[Name]"></textarea>
    </div>
    <div v-else-if="Type == 'date'" class="form-group">
    <label :for="Name">{{Description}}</label>
      <input type="date" :id="Name" v-model="formData[Name]" class="form-control input-sm">
    </div>
    <div v-else-if="Type == 'integer'" class="form-group">
      <label :for="Name">{{Description}}</label>
      <input type="number" :id="Name" v-model="formData[Name]" class="form-control input-sm">
    </div>
    <div v-else-if="Type == 'boolean'" class="form-group">
      <label class="form-check-label">\
        <input :id="Name" v-model="formData[Name]" type="checkbox" class="form-check-input">\
        {{Description}}\
        <span class="checkmark"></span>\
      </label>\
    </div>
    <div v-else-if="Type == 'node'" class="form-group">
      <label :for="Name">{{Description}}</label>
      <node-selector :id="Name" v-model="formData[Name]"></node-selector>
    </div>
    <div v-else-if="Type == 'choice'" class="form-group">
      <label :for="Name">{{Description}}</label>
      <template>
        <select :id="Name" v-model="formData[Name]" class="form-control custom-select">
          <option v-for="(option, index) in Values" :value="option.Value">{{ option.Value }} ({{ option.Description }})</option>
        </select>
      </template>
    </div>
    <div v-else-if="Type == 'group'" class="form-group">
      <label :for="Name">{{Description}}</label>
      <template>
        <item v-for="i in item" v-bind="i" :key="i.Name"></item>
      </template>
    </div>
  </div>`
})

Vue.component('workflow-params', {
  props: [
    'workflow'
  ],

  data() {
  	return {
      formData: {
      }
    }
  },

  provide() {
    return {
      formData: this.formData
    }
  },

  inject: [
    'result'
  ],

  template: `
    <form v-if="workflow.Parameters" @submit.prevent="submit">
      <h1><span class="workflow-title">{{workflow.Name}}</span></h1>
      <item v-for="item in workflow.Parameters" v-bind="item" :key="item.Name"></item>
      <button type="submit" id="execute" class="btn btn-primary">Execute</button>\
    </form>
  `,

  methods: {

    submit: function() {
      var self = this;
      var source = "(" + this.workflow.Source + ")";
      var f = eval(source);
      if (typeof f !== "function") {
        throw "Source is not a function";
      }
      var args = [];
      for (var i in this.workflow.Parameters) {
        var param = this.workflow.Parameters[i];
        args.push(this.formData[param.Name]);
      }
      var promise = f.apply({}, args);
      promise.then(function (result) {
        self.result.value = result;
        if (typeof result == "object") {
          if (result.nodes && result.edges) {
            var g = topologyComponent.graph;
            for (var n in result.nodes) {
              var node = result.nodes[n];
              g.addNode(node.ID, node.Host, node.Metadata);
            }
            for (var e in result.edges) {
              var edge = result.edges[e];
              g.addEdge(edge.ID, edge.Host, edge.Metadata, g.nodes[edge.Parent], g.nodes[edge.Child]);
            }
          }
        }
      }).catch(function (e) {
        self.result.value = e.toString();
      })
    }
  }
})

Vue.component('workflow-call', {
  mixins: [apiMixin],

  data() {
  	return {
      "currentWorkflow": {},
      "result": {
        "value": null
      },
      "workflows": {}
    }
  },

  provide() {
    return {
      result: this.result
    }
  },

  template: `
    <div class="form-group">
      <div class="form-group">
        <label for="workflow">Workflows</label>
        <select id="workflow" v-model="currentWorkflow" class="form-control custom-select">
          <option selected :value="{}">Select a workflow</option>\
          <option v-for="(workflow, id) in workflows" :value="workflow">{{ workflow.Name }} ({{ workflow.Description }})</option>
        </select>
      </div>
      <div class="form-group">
        <workflow-params :workflow="currentWorkflow"></workflow-params>
      </div>
      <div class="form-group" v-if="result.value">
        <h1><span class="workflow-title">Result</span></h1>
        <div class="form-group">
          <textarea readonly id="workflow-output" type="text" class="form-control input-sm" rows="5" v-model="result.value" v-if="result.value && (typeof result.value) != 'object'"></textarea>
          <object-detail v-else-if="typeof result.value == 'object'" id="workflow-output" :object="result.value"></object-detail>\
        </div>
      </div>
    </div>
  `,

  watch: {
    'currentWorkflow': function () {
      this.result.value = undefined
    }
  },

  created: function() {
    var self = this;
    self.workflowAPI.list()
      .then(function(data) {
        self.workflows = data;

        // setting default
        for (var i in this.workflow.Parameters) {
          var param = this.workflow.Parameters[i];
          param.Values = param.Default;
        }
      })
      .catch(function (e) {
        if (e.status === 405) { // not allowed
          return $.Deferred().promise([]);
        }
        self.$error({message: 'Error while listing workflows: ' + e.responseText});
        return e;
      });
  }

})
