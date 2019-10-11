package main

import (
	"github.com/skydive-project/skydive/graffiti/graph"
	pr "github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology"
	"github.com/skydive-project/skydive/topology/probes"
)

type probe struct {
	ctx probes.Context
}

// Start the probe
func (p *probe) Start() error {
	p.ctx.Logger.Debug("Starting sample probe")
	n, err := p.ctx.Graph.NewNode(
		graph.Identifier("SampleProbe"),
		graph.Metadata{"Type": "device", "Name": "sample"},
	)
	if err != nil {
		return err
	}
	_, err = topology.AddOwnershipLink(p.ctx.Graph, p.ctx.RootNode, n, nil)
	return err
}

// Stop the probe
func (p *probe) Stop() {
	p.ctx.Logger.Info("Stopping sample probe")
}

// NewAgentProbe returns an new sample probe handler
func NewAgentProbe(ctx probes.Context, bundle *pr.Bundle) (probes.Handler, error) {
	return &probe{ctx: ctx}, nil
}

// Register registers graph metadata decoders
func Register() {
}

func main() {
}
