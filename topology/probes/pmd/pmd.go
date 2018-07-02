package pmd

import (
        "time"
        "os/exec"
        "sync/atomic"
        "sync"
        "github.com/skydive-project/skydive/common"
        "github.com/skydive-project/skydive/logging"
        "github.com/skydive-project/skydive/topology/graph"
)

type PMDProbe struct {
        graph.DefaultGraphListener
        quit      chan bool
        graph     *graph.Graph
        state        int64
        wg        sync.WaitGroup
}
func (o *PMDProbe)  OnNodeAdded(n *graph.Node) {
        if tp, _ := n.GetFieldString("Name"); tp == "ovs-system" {
                atomic.StoreInt64(&o.state, common.RunningState)
                o.wg.Done()
        }
}


func (o *PMDProbe) CoverageUpdate() error {
        o.graph.Lock()
        defer o.graph.Unlock()
        commandLine := []string{"ovs-appctl","coverage/show"}
        lines, err := launchOnSwitch(commandLine)
        if err != nil {
                logging.GetLogger().Infof("coverage/show failed with %s %s: ",  lines, err.Error())
                return err
        }
        node := o.graph.LookupFirstNode(graph.Metadata{"Name": "ovs-system"})
        if node != nil {
                o.graph.AddMetadata(node, "coverage", lines)
        }
        intfs := o.graph.GetNodes(graph.Metadata{
                "Type":    "ovsbridge",
                "Datapath Type": "netdev",
        })
        if len(intfs) != 0 {
                o.PMDUpdate()
        }
        return nil
}

func (o *PMDProbe) PMDUpdate() error {
        commandLine := []string{"ovs-appctl", "dpif-netdev/pmd-stats-show"}
        lines, err := launchOnSwitch(commandLine)
        if err != nil {
                logging.GetLogger().Infof("pmd-stats-show failed with %s %s: ",  lines, err.Error())
                return err
        }
        node := o.graph.LookupFirstNode(graph.Metadata{"Name": "ovs-system"})
        if node != nil {
                o.graph.AddMetadata(node, "pmd", lines)
        }
        return nil
}



func (m *PMDProbe) Start() {
        m.graph.AddEventListener(m)
        m.wg.Add(1)
Ready:  
        for {   
                switch atomic.LoadInt64(&m.state) {
                case common.RunningState:
                        break Ready
                case common.StoppingState, common.StoppedState:
                        m.wg.Wait()
                        //time.Sleep(5 * time.Second)
                }
        }
        m.CoverageUpdate()

        go func() {
                ticker := time.NewTicker(time.Duration(10) * time.Second)
                defer ticker.Stop()

                for {   
                        select {
                        case <-m.quit:
                                return
                        case <-ticker.C:
                                m.CoverageUpdate()
                        }
                }
        }()
}


func (m *PMDProbe) Stop() {
        m.graph.RemoveEventListener(m)
        atomic.StoreInt64(&m.state, common.StoppedState)
        m.quit <- true
}


func launchOnSwitch(cmd []string) (string, error) {
        command := exec.Command(cmd[0], cmd[1:]...)
        bytes, err := command.CombinedOutput()
        if err == nil {
                return string(bytes), nil
        }
        return string(bytes), err
}

func NewPMDProbe(g *graph.Graph) *PMDProbe {

        return &PMDProbe{
                graph:  g,
                quit:      make(chan bool),
                state:        common.StoppingState,
        }
}

