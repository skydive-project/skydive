package probes

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/logging"
)

var (
	// ErrNotStopped is used when a not stopped probe is started
	ErrNotStopped = errors.New("service is not stopped")
)

type handler interface {
	Do(ctx context.Context, wg *sync.WaitGroup) error
}

type serviceManager struct {
	state         int64
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
	if !atomic.CompareAndSwapInt64(&sm.state, common.StoppedState, common.StartingState) {
		return ErrNotStopped
	}

	ctx, cancel := context.WithCancel(ctx)
	sm.cancel = cancel

	go func() {
		for state := atomic.LoadInt64(&sm.state); state != common.StoppingState && state != common.StoppedState; state = atomic.LoadInt64(&sm.state) {
			if err := sm.handler.Do(ctx, &sm.wg); err != nil {
				logging.GetLogger().Error(err)
			} else {
				state = common.RunningState
				if !atomic.CompareAndSwapInt64(&sm.state, common.StartingState, common.RunningState) {
					return
				}

				sm.wg.Wait()
			}

			after := time.After(sm.retryInterval)
			if atomic.CompareAndSwapInt64(&sm.state, state, common.StartingState) {
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
	state := atomic.LoadInt64(&sm.state)
	if state == common.StoppedState || state == common.StoppingState {
		return
	}

	if !atomic.CompareAndSwapInt64(&sm.state, state, common.StoppingState) {
		return
	}

	sm.cancel()

	sm.wg.Wait()

	atomic.StoreInt64(&sm.state, common.StoppedState)
}

// ProbeWrapper wraps a probe so that it tries to reconnect if the connection
// to the daemon is lost
type ProbeWrapper struct {
	sm *serviceManager
}

// Start the probe
func (p *ProbeWrapper) Start() {
	if err := p.sm.Start(context.Background()); err != nil {
		logging.GetLogger().Error(err)
	}
}

// Stop the probe
func (p *ProbeWrapper) Stop() {
	p.sm.Stop()
}

// NewProbeWrapper returns a new probe wrapper
func NewProbeWrapper(handler handler) *ProbeWrapper {
	return &ProbeWrapper{sm: newServiceManager(handler, 3*time.Second)}
}
