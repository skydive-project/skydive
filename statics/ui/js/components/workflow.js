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
      <textarea :id="Name" type="text" class="form-control input-sm" v-model="formData[Name]"></textarea>
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
  </div>
  `,

  created() {
    if (this.Default) this.formData[this.Name] = this.Default
  },
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
    'result', 'toggleResultDisplay'
  ],

  template: `
    <form v-if="workflow.Parameters" @submit.prevent="submit">
      <div class="workflow-abstract">
        {{ workflow.Abstract }}
      </div>
      <vue-markdown class="workflow-description" v-bind:source="workflow.Description" v-bind:postrender="beautifyDesc"/>
      <h1><span class="workflow-title">Inputs</span></h1>
      <item v-for="item in workflow.Parameters" v-bind="item" :key="item.Name"></item>
      <button type="submit" id="execute" class="btn btn-primary">Execute</button>\
    </form>
  `,

  methods: {

    beautifyDesc: function(out) {
      var i = 0
      var href = function() {
        i++;
        return '<h1 onclick="$(\'#wf-arrow-' + i + '\').toggleClass(\'down\')" \
          class="workflow-desc-title collapse-header" role="button" data-toggle="collapse" href=".desc-' + i + '">\
          <span class="pull-left" style="padding-right: 15px">\
            <i id="wf-arrow-' + i + '" class="glyphicon glyphicon-chevron-right rotate"></i>\
          </span>';
      }
      out = out.replace(/<h1>/g, href);

      i = 0
      var target = function(match, m1) {
        i++;
        return '</h1><' + m1 + ' class="collapse desc-' + i + '">';
      }
      return out.replace(/<\/h1>\s*<([a-z]*)>/gm, target);
    },

    submit: function() {
      this.toggleResultDisplay(true);
      var self = this;

      var args = [];
      for (var i in this.workflow.Parameters) {
        var param = this.workflow.Parameters[i];
        args.push(this.formData[param.Name]);
      }

      let url = "/api/workflow/" + this.workflow.UUID + "/call"
      $.ajax({
        dataType: "json",
        url: url,
        data: JSON.stringify({"Params": args}),
        contentType: "application/json",
        method: "POST",
      })
      .then(function(result) {
        self.result.value = result
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
      })
      .fail(function(e){
        self.result.value = e.toString()
      })
    }
  }
})

Vue.component('workflow-call', {
  mixins: [apiMixin, notificationMixin],

  data() {
  	return {
      "visible": false,
      "currentWorkflow": {},
      "result": {
        "value": null
      },
      "workflows": {},
      "display": false,
      "fileContent": "",
    }
  },

  provide() {
    return {
      result: this.result,
      toggleResultDisplay: this.toggleResultDisplay
    }
  },

  template: `
    <div>
      <div class="form-group">
        <div class="form-group">
          <select id="workflow" v-model="currentWorkflow" class="form-control custom-select">
            <option selected :value="{}">Select/Upload a workflow</option>\
            <option value="__upload__">New workflow</option>\
            <option v-for="(workflow, id) in workflows" :value="workflow">{{ workflow.Name }} - {{ workflow.Title }}</option>
          </select>
        </div>

        <div v-if="currentWorkflow == '__upload__'">
        <input type="file" id="file_selector" @change="openfile($event)" style="display:none;">
        <button type="button"\
                id="create-workflow"\
                class="btn btn-primary"\
                @click="uploadFile">
          Upload
        </button>
        </div>

        <hr/>

        <div class="form-group">
          <workflow-params :workflow="currentWorkflow"></workflow-params>
        </div>
        <div class="form-group" v-if="display">
          <h1><span class="workflow-title">Result</span></h1>
          <div class="form-group" v-if="result.value">
            <textarea readonly id="workflow-output" type="text" class="form-control input-sm" rows="5" v-model="result.value" v-if="result.value && (typeof result.value) != 'object'"></textarea>
            <object-detail v-else-if="typeof result.value == 'object'" id="workflow-output" :object="result.value"></object-detail>\
          </div>
          <div class="form-group" v-else>
            <i class="fa fa-circle-o-notch fa-spin fa-2x fa-fw"></i>
          </div>
        </div>
      </div>
    </div>
  `,

  watch: {
    'currentWorkflow': function () {
      this.toggleResultDisplay(false);
    }
  },

  created: function() {
    this.loadWorkflows();
  },

  methods: {
    loadWorkflows: function() {
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
    },

    toggleResultDisplay: function(status) {
      this.display = status;
      this.result.value = undefined;
    },

    create: function() {
      var self = this;
      if (this.fileContent == "") {
        this.$error({message: "Select a file first"});
        return;
      }
      $.ajax({
        dataType: "json",
        url: "/api/workflow",
        data: self.fileContent,
        contentType: "application/yaml",
        method: "POST",
      })
      .then(function() {
        self.$success({message: "workflow created"});
        self.loadWorkflows();
      })
      .fail(function(e){
        self.$error({message: "workflow create error: " + e.responseText});
      })
    },

    openfile: function(event) {
      var self = this;
      file = event.target.files[0];
      r = new FileReader();
      r.onload = function(e) {
        self.fileContent = e.target.result;
        self.create();
      };
      r.readAsText(file);
    },

    uploadFile: function() {
      document.getElementById('file_selector').click();
    },
  }
})
