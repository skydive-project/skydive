package hub

import (
	"context"
	"sync"
	"time"

	etcdclient "github.com/skydive-project/skydive/graffiti/etcd/client"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/websocket"
)

type clusterPeering struct {
	currentPeer    int
	clusterName    string
	logger         logging.Logger
	masterElection etcdclient.MasterElection
	peers          *websocket.ClientPool
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
}

func (p *clusterPeering) OnStartAsMaster() {
	p.connect()
}

func (p *clusterPeering) OnSwitchToMaster() {
	p.connect()
}

func (p *clusterPeering) OnStartAsSlave() {
}

func (p *clusterPeering) OnSwitchToSlave() {
	if p.cancel != nil {
		p.cancel()
		p.wg.Wait()
		p.cancel = nil
	}
}

func (p *clusterPeering) OnNewMaster(c websocket.Speaker) {
}

func (p *clusterPeering) connect() {
	websocketElection := websocket.NewMasterElection(p.peers)
	websocketElection.AddEventHandler(p)

	p.ctx, p.cancel = context.WithCancel(context.Background())
	speakers := p.peers.GetSpeakers()
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()

		for {
			select {
			case <-p.ctx.Done():
				return
			default:
				speaker := speakers[p.currentPeer]
				if err := speaker.Connect(p.ctx); err == nil && p.masterElection.IsMaster() {
					p.logger.Infof("Peered to cluster %s", p.clusterName)
					speaker.Run()
				}

				p.currentPeer = (p.currentPeer + 1) % len(speakers)
				if p.currentPeer == 0 {
					p.logger.Warningf("Failed to peer with cluster %s, retrying in 3 seconds", p.clusterName)
					time.Sleep(3 * time.Second)
				}
			}
		}
	}()
}
