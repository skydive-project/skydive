package probes

import (
	"context"
	"sync"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/probe"
)

type handler interface {
	Do(ctx context.Context, wg *sync.WaitGroup) error
}

// serviceManager manages the state of a service using the following
// state machine: Stopped -> Starting -> Running -> Stopping
// Typical usage is the following: when a service is started
// is goes from Stopped to Starting. If the service successfully
// started, it will go into the Running state. Otherwise, it will
// try to reconnect. If the connection is lost, the service will return
// in Starting mode.
type serviceManager struct {
	state         common.ServiceState
	handler       handler
	retryInterval time.Duration
	wg            sync.WaitGroup
	cancel        context.CancelFunc
}

func newServiceManager(handler handler, retryInterval time.Duration) *serviceManager {
	return &serviceManager{
		handler:       handler,
		state:         common.StoppedState,
		retryInterval: retryInterval,
	}
}

// Start the daemon
func (sm *serviceManager) Start(ctx context.Context) error {
	if !sm.state.CompareAndSwap(common.StoppedState, common.StartingState) {
		return probe.ErrNotStopped
	}

	ctx, cancel := context.WithCancel(ctx)
	sm.cancel = cancel

	go func() {
		for state := sm.state.Load(); state != common.StoppingState && state != common.StoppedState && ctx.Err() != context.Canceled; state = sm.state.Load() {
			if err := sm.handler.Do(ctx, &sm.wg); err != nil {
				logging.GetLogger().Error(err)
			} else {
				state = common.RunningState
				if !sm.state.CompareAndSwap(common.StartingState, common.RunningState) {
					return
				}

				sm.wg.Wait()
			}

			after := time.After(sm.retryInterval)
			if sm.state.CompareAndSwap(state, common.StartingState) {
				select {
				case <-ctx.Done():
					return
				case <-after:
				}
			}
		}
	}()

	return nil
}

func (sm *serviceManager) Stop() {
	state := sm.state.Load()
	if state == common.StoppedState || state == common.StoppingState {
		return
	}

	if !sm.state.CompareAndSwap(state, common.StoppingState) {
		return
	}

	sm.cancel()

	sm.wg.Wait()

	sm.state.Store(common.StoppedState)
}

// ProbeWrapper wraps a probe so that it tries to reconnect if the connection
// to the daemon is lost
type ProbeWrapper struct {
	sm *serviceManager
}

// GetStatus returns the status of the probe
func (p *ProbeWrapper) GetStatus() interface{} {
	return &probe.ServiceStatus{
		Status: common.ServiceState(p.sm.state),
	}
}

// Start the probe
func (p *ProbeWrapper) Start() error {
	return p.sm.Start(context.Background())
}

// Stop the probe
func (p *ProbeWrapper) Stop() {
	p.sm.Stop()
}

// NewProbeWrapper returns a new probe wrapper
func NewProbeWrapper(handler handler) *ProbeWrapper {
	return &ProbeWrapper{sm: newServiceManager(handler, 3*time.Second)}
}
