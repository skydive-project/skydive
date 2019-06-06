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
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/skydive-project/skydive/logging"
)

// ClientAPIVersion Client API version used
const ClientAPIVersion = "1.18"

// newDockerClient creates new Docker API client
func newDockerClient() (*client.Client, error) {
	dockerURL := "unix:///var/run/docker.sock"
	defaultHeaders := map[string]string{"User-Agent": "skydive-test"}
	dockerClient, err := client.NewClient(dockerURL, ClientAPIVersion, nil, defaultHeaders)
	if err != nil {
		return nil, fmt.Errorf("failed to create client to Docker daemon: %s", err)
	}
	if _, err := dockerClient.ServerVersion(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to connect to Docker daemon: %s", err)
	}
	return dockerClient, nil
}

// dockerExec performs docker API equivalent of "docker exec" CLI command. Exec action is performed on container <containerName>
// with execution command split to base command and parameters in <cmd> parameter.
func dockerExec(containerName string, cmd []string) (string, error) {
	return dockerExecTimeouted(containerName, cmd, 1000000*time.Hour)
}

// dockerExecTimeouted does the same as dockerExec function, but the docker exec run can be timeouted after timeout <timeout>.
func dockerExecTimeouted(containerName string, cmd []string, timeout time.Duration) (string, error) {
	// get docker client
	dockerClient, err := newDockerClient()
	if err != nil {
		return "", fmt.Errorf("can't create docker client due to: %v", err)
	}
	defer func() {
		if closeErr := dockerClient.Close(); closeErr != nil {
			logging.GetLogger().Errorf("can't close docker client due to: %v", closeErr)
		}
	}()

	// create exec instance
	execConfig := types.ExecConfig{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
		Cmd:          cmd,
	}
	execInstance, err := dockerClient.ContainerExecCreate(context.Background(), containerName, execConfig)
	if err != nil {
		return "", fmt.Errorf("can't create exec instance for command %v in container %v due to: %v", cmd, containerName, err)
	}

	// attach to created exec instance to be able to read output of started exec instance
	att, err := dockerClient.ContainerExecAttach(context.Background(), execInstance.ID, execConfig)
	if err != nil {
		return "", fmt.Errorf("can't attach to created exec instance (command %v) in container %v due to: %v", cmd, containerName, err)
	}
	defer att.Close()
	execStartConfing := types.ExecStartCheck{
		Detach: false,
		Tty:    false,
	}

	// starting execution of exec instance
	err = dockerClient.ContainerExecStart(context.Background(), execInstance.ID, execStartConfing)
	if err != nil {
		return "", fmt.Errorf("can't start created exec instance (command %v) in container %v due to: %v", cmd, containerName, err)
	}

	// waiting until execution finish
	timeoutChan := time.After(timeout)
	var execInfo types.ContainerExecInspect
	for {
		execInfo, err = dockerClient.ContainerExecInspect(context.Background(), execInstance.ID)
		if err != nil {
			logging.GetLogger().Debugf("can't check ending of exec instance(command %v) due to: %v", cmd, err)
		}
		if !execInfo.Running || len(timeoutChan) > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	// getting output of finished exec instance
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(att.Reader)
	if err != nil {
		return "", fmt.Errorf("can't read output of started exec instance(command %v) in container %v due to: %v", cmd, containerName, err)
	}
	content := buf.String()

	// checking remote command failure
	if execInfo.ExitCode != 0 {
		return "", fmt.Errorf("exit code %v", execInfo.ExitCode)
	}

	return content, nil
}
