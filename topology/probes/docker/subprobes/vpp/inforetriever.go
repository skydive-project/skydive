// +build docker_vpp,linux

// Copyright (c) 2019 PANTHEON.tech s.r.o.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vpp

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
)

const (
	vppCLIShowVersion  = "show version"
	vppCLIShowHardware = "show hardware"
	vppCLIShowMemif    = "show memif"
	vppInfoSeparator   = "###"
)

// dockerDataRetrieval retrieves all needed data about VPPs in all containers. Data are returned in map, where
// map[container name][vpp CLI command] = vpp CLI command output. Reason for this method is to optimize data
// retrieval (solving retrieving in parallel and with minimal docker exec calls due to its massive overhead when
// container count rises)
func (p *Subprobe) dockerDataRetrieval(containerNames map[string]*containerInfo) map[string]map[string]string {
	type dockerExecResult struct {
		containerName string
		vppCLICommand string
		output        string
	}
	dataChan := make(chan dockerExecResult, 100)

	// launch goroutine that merges results from all worker goroutines
	dockerExecResults := make(map[string]map[string]string) // [containerName][vpp CLI command] = vpp CLI command output
	ctx, cancelMerger := context.WithCancel(context.Background())
	var mergerWG sync.WaitGroup
	mergerWG.Add(1)
	go func(results *map[string]map[string]string) {
		defer mergerWG.Done()
		addResult := func(data dockerExecResult) {
			if dockerExecResults[data.containerName] == nil {
				dockerExecResults[data.containerName] = make(map[string]string)
			}
			dockerExecResults[data.containerName][data.vppCLICommand] = data.output
		}
		for {
			select {
			case data := <-dataChan:
				addResult(data)
			case <-ctx.Done():
				for i := 0; i < len(dataChan); i++ {
					addResult(<-dataChan)
				}
				return
			}
		}
	}(&dockerExecResults)

	// launching parallel-working workers that retrieve all data from all VPPs in all docker containers
	var workersWG sync.WaitGroup
	for containerName := range containerNames {
		workersWG.Add(1)
		go func(containerName string) {
			defer workersWG.Done()
			// getting all VPP Info in one docker exec call, split it and forward it to merger goroutine
			// (the big bottleneck is the overhead in docker exec call (cmd executed inside container is relatively time cheap) -> minimizing docker exec calls)
			// NOTE: don't know why, but go docker client is 3 times slower as Linux cmd line "docker exec" command (tested parallel execution for 11 containers),
			// but we can't use direct Linux cmd line, because Skydive agent is running also in container -> can't get out of it (this is totally against container
			// principles->hard/impossible to do)
			vppInfoStr, err := dockerExec(containerName, p.vppScriptCmd())
			if err != nil {
				logging.GetLogger().Debugf("failed to retrieve vpp information from container %v due to: %v\n", containerName, err)
			} else { // running VPP detected
				logging.GetLogger().Debugf("VPP Probe: container %v, running VPP detected\n", containerName)

				// extract each data item from string and send it to merger goroutine
				infoOrder := []string{vppCLIShowVersion, vppCLIShowHardware, vppCLIShowMemif}
				infoData := strings.Split(vppInfoStr, vppInfoSeparator)
				if len(infoData) != len(infoOrder) {
					logging.GetLogger().Errorf("Count of data items retrieved from container %v doesn't correspond to expected "+
						"data item count (expected=%v, got=%v, full data string=%v)", containerName, len(infoOrder), len(infoData), vppInfoStr)
					return
				}
				for i, data := range infoData {
					if len(strings.TrimPrefix(strings.TrimSpace(data), "\n")) != 0 { // only relevant data are used (empty spaced, 2 lines empty spaced data are ignored)
						// send docker exec result to merger goroutine
						dataChan <- dockerExecResult{
							containerName: containerName,
							vppCLICommand: infoOrder[i],
							output:        data,
						}
					}
				}
			}
		}(containerName)
	}

	workersWG.Wait() // wait for all docker exec commands to finish
	cancelMerger()   // signal for merger to process what is in channel and stop
	mergerWG.Wait()  // wait for merging of all results from docker exec commands

	return dockerExecResults
}

// vppScriptCmd creates shell script to extract all VPP information from locally installed VPP (script should run inside container where VPP is installed)
func (p *Subprobe) vppScriptCmd() []string {
	script := `
version=` + "`" + p.vppCommandFromOSLevel(vppCLIShowVersion) + "`" + `
retVal=$?
if [ $retVal -eq 0 ]; then
  echo "$version"
  echo "` + vppInfoSeparator + `"
  hardware=` + "`" + p.vppCommandFromOSLevel(vppCLIShowHardware) + "`" + `
  hwretVal=$?
  if [ $hwretVal -eq 0 ]; then
    echo "$hardware"
  fi
  echo "` + vppInfoSeparator + `"
  memif=` + "`" + p.vppCommandFromOSLevel(vppCLIShowMemif) + "`" + `
  miretVal=$?
  if [ $miretVal -eq 0 ]; then
    echo "$memif"
  fi
else
  echo "` + vppInfoSeparator + `"
  echo "` + vppInfoSeparator + `"
fi
return $retVal
`
	return []string{"sh", "-c", script}
}

// vppCommandFromOSLevel transforms vpp command for VPP CLI to vpp command for OS shell command in OS where is VPP running as process.
func (p *Subprobe) vppCommandFromOSLevel(cliCommand string) string {
	socket := config.GetString("agent.topology.docker.vpp.cliconnect")
	if socket == "" { // using default VPP CLI socket (/run/vpp/cli.sock)
		return fmt.Sprintf("vppctl %s", cliCommand)
	}
	return fmt.Sprintf("vppctl -s %s %s", socket, cliCommand)
}
