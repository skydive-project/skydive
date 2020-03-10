package probes

import (
	"context"
	"sync"
	"time"

	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/service"
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
	state         service.State
	handler       handler
	retryInterval time.Duration
	wg            sync.WaitGroup
	cancel        context.CancelFunc
}

func newServiceManager(handler handler, retryInterval time.Duration) *serviceManager {
	return &serviceManager{
		handler:       handler,
		state:         service.StoppedState,
		retryInterval: retryInterval,
	}
}

// Start the daemon
func (sm *serviceManager) Start(ctx context.Context) error {
	if !sm.state.CompareAndSwap(service.StoppedState, service.StartingState) {
		return probe.ErrNotStopped
	}

	ctx, cancel := context.WithCancel(ctx)
	sm.cancel = cancel

	go func() {
		for state := sm.state.Load(); state != service.StoppingState && state != service.StoppedState && ctx.Err() != context.Canceled; state = sm.state.Load() {
			if err := sm.handler.Do(ctx, &sm.wg); err != nil {
				logging.GetLogger().Error(err)
			} else {
				state = service.RunningState
				if !sm.state.CompareAndSwap(service.StartingState, service.RunningState) {
					return
				}

				sm.wg.Wait()
			}

			after := time.After(sm.retryInterval)
			if sm.state.CompareAndSwap(state, service.StartingState) {
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
	if state == service.StoppedState || state == service.StoppingState {
		return
	}

	if !sm.state.CompareAndSwap(state, service.StoppingState) {
		return
	}

	sm.cancel()

	sm.wg.Wait()

	sm.state.Store(service.StoppedState)
}

// ProbeWrapper wraps a probe so that it tries to reconnect if the connection
// to the daemon is lost
type ProbeWrapper struct {
	sm *serviceManager
}

// GetStatus returns the status of the probe
func (p *ProbeWrapper) GetStatus() interface{} {
	return &probe.ServiceStatus{
		Status: service.State(p.sm.state),
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
