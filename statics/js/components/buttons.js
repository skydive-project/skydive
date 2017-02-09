/* jshint multistr: true */

Vue.component('button-state', {

  props: {

    value: {
      type: Boolean,
      required: true,
    },

    enabledText: {
      type: String,
      required: true,
    },

    disabledText: {
      type: String,
      required: true,
    }

  },

  template: '\
    <button type="button" class="btn btn-default"\
            :class="{\'active\': value}"\
            @click="change">\
      <span v-if="value">{{enabledText}}</span>\
      <span v-else>{{disabledText}}</span>\
    </button>\
  ',

  methods: {

    change: function() {
      this.$emit('input', !this.value);
    }

  }

});

Vue.component('button-dropdown', {

  props: {

    // Button text
    text: {
      type: String,
    },

    // Button css classes
    bClass: {},

    // Can be also up/down
    // auto will calculate if there is enough
    // room to put the menu down, if not it will
    // be up.
    position: {
      type: String,
      default: "auto",
    },

    // Hide the menu after clicking on some
    // element in it
    autoClose: {
      type: Boolean,
      default: true,
    },

  },

  template: '\
    <div class="btn-group" :id="id" :class="{\'open\': open, \'dropup\': dropup}">\
      <button class="btn btn-default dropdown-toggle"\
              :class="bClass"\
              @click="toggle"\
              aria-haspopup="true"\
              :aria-expanded="open">\
          <slot name="button-text">\
            {{text}}\
          </slot>\
      </button>\
      <ul class="dropdown-menu" v-if="open" @click="itemSelected">\
        <slot></slot>\
      </ul>\
    </div>\
  ',

  data: function() {
    return {
      open: false,
      // generate an unique id for this dropdown
      id: "btn-group-" + Math.random().toString(36).substr(2, 9),
      dropup: false,
    };
  },

  mounted: function() {
    var self = this;
    // close the popup if we click elsewhere
    // from the target search if any parent has the id
    $(document).on('click', function(event) {
      if (self.open === false)
        return;
      if ($(event.target).closest('#'+self.id).length === 0) {
        self.open = false;
      }
    });
  },

  methods: {

    toggle: function() {
      this.open = !this.open;
      if (this.open === true) {
        var self = this;
        this.$nextTick(function () {
          switch (this.position) {
            case "up":
              this.dropup = true;
              break;
            case "down":
              this.dropup = false;
              break;
            case "auto":
              var button = $(self.$el),
                  bottomPosition = button.offset().top + button.height(),
                  menuHeight = button.find('.dropdown-menu').height(),
                  windowHeight = $(window).height();
              if (menuHeight > windowHeight - bottomPosition) {
                this.dropup = true;
              } else {
                this.dropup = false;
              }
              break;
          }
        });
      }
    },

    itemSelected: function() {
      if (this.autoClose) {
        this.toggle();
      }
    },

  },

});
