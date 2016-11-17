var timesliderLastTime = 0;
var timesliderIntervalId;
var slowMotionEffectId;
function SetupTimeSlider() {
  var slowMotion = function() {
    $('body').addClass('slowmotion-effect-in');
    $('body').removeClass('slowmotion-effect-out');
    if (slowMotionEffectId) {
        clearTimeout(slowMotionEffectId);
    }
    slowMotionEffectId = setTimeout(function(){
      $('body').removeClass('slowmotion-effect-in');
      $('body').addClass('slowmotion-effect-out');
    }, 1000);
  };

  var changeTime = function() {
    var time = new Date();
    time.setMinutes(time.getMinutes() + slider.getValue());
    time.setSeconds(0);

    var at = time.getTime();

    if (at == timesliderLastTime)
      return;

    topologyLayout.SyncRequest(at);

    if (timesliderIntervalId) {
      clearInterval(timesliderIntervalId);
      timesliderIntervalId = null;
    }

    timesliderIntervalId = setInterval(function() {
      slider.setValue(slider.getValue() - 1);
    }, 60 * 1000);

    timesliderLastTime = at;
  };

  var slider = new Slider("#timeslider", {
    tooltip: 'always',
    formatter: function(value) {
  		return -value + ' min. ago';
  	}
  })
  .on('slide', function() {
    slowMotion();
  })
  .on('change', function() {
    if (!slider.isEnabled())
      return;

    slowMotion();
    changeTime();
  });

  $("[name='live-switch']").bootstrapSwitch({
    onSwitchChange: function(event, state) {
      if (state && topologyLayout.live === false) {
        topologyLayout.SyncRequest(null);
      }

      if (state) {
        slider.disable();
        slider.setValue(0);

        if (timesliderIntervalId) {
          clearInterval(timesliderIntervalId);
          timesliderIntervalId = null;
        }

        $(".flow-ops-panel").show();
      }
      else {
        slider.enable();

        changeTime();

        $(".flow-ops-panel").hide();
      }

      topologyLayout.live = state;
      return true;
    }
  });
  $("[name='live-switch']").bootstrapSwitch('state', true, false);
}
