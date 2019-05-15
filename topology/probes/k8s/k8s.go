/*
 * Copyright 2018 Red Hat
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy ofthe License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specificlanguage governing permissions and
 * limitations under the License.
 *
 */

package k8s

import (
	"fmt"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// NewConfig returns a new Kubernetes configuration object
func NewConfig(kubeconfigPath string) (*rest.Config, *clientcmd.ClientConfig, error) {
	var err error
	var cc *rest.Config

	if kubeconfigPath == "" {
		cc, err := rest.InClusterConfig()
		if err == nil {
			return cc, nil, nil
		}
	}

	kubeconfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{})

	cc, err = kubeconfig.ClientConfig()
	if err == nil {
		return cc, &kubeconfig, nil
	}

	kubeconfig = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{})

	cc, err = kubeconfig.ClientConfig()
	if err == nil {
		return cc, &kubeconfig, nil
	}

	return nil, nil, fmt.Errorf("Failed to load Kubernetes config: %s", err)
}

// NewK8sProbe returns a new Kubernetes probe
func NewK8sProbe(g *graph.Graph) (*Probe, error) {
	kubeconfigPath := config.GetString("analyzer.topology.k8s.config_file")
	enabledSubprobes := config.GetStringSlice("analyzer.topology.k8s.probes")

	clientconfig, kubeconfig, err := NewConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(clientconfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kubernetes client: %s", err)
	}

	subprobeHandlers := map[string]SubprobeHandler{
		"cluster": newClusterProbe,
	}

	InitSubprobes(enabledSubprobes, subprobeHandlers, kubeconfig, g, Manager)

	subprobeHandlers = map[string]SubprobeHandler{
		"configmap":             newConfigMapProbe,
		"container":             newContainerProbe,
		"cronjob":               newCronJobProbe,
		"daemonset":             newDaemonSetProbe,
		"deployment":            newDeploymentProbe,
		"endpoints":             newEndpointsProbe,
		"ingress":               newIngressProbe,
		"job":                   newJobProbe,
		"namespace":             newNamespaceProbe,
		"networkpolicy":         newNetworkPolicyProbe,
		"node":                  newNodeProbe,
		"persistentvolume":      newPersistentVolumeProbe,
		"persistentvolumeclaim": newPersistentVolumeClaimProbe,
		"pod":                   newPodProbe,
		"replicaset":            newReplicaSetProbe,
		"replicationcontroller": newReplicationControllerProbe,
		"secret":                newSecretProbe,
		"service":               newServiceProbe,
		"statefulset":           newStatefulSetProbe,
		"storageclass":          newStorageClassProbe,
	}

	InitSubprobes(enabledSubprobes, subprobeHandlers, clientset, g, Manager)

	linkerHandlers := []LinkHandler{
		newContainerDockerLinker,
		newDeploymentPodLinker,
		newDeploymentReplicaSetLinker,
		newPodContainerLinker,
		newPodConfigMapLinker,
		newPodSecretLinker,
		newHostNodeLinker,
		newNodePodLinker,
		newIngressServiceLinker,
		newNetworkPolicyLinker,
		newServiceEndpointsLinker,
		newServicePodLinker,
		newStatefulSetPodLinker,
		newPodPVCLinker,
		newPVPVCLinker,
		newStorageClassPVCLinker,
		newStorageClassPVLinker,
	}

	linkers := InitLinkers(linkerHandlers, g)

	verifiers := []probe.Probe{}

	probe := NewProbe(g, Manager, subprobes[Manager], linkers, verifiers)

	probe.AppendClusterLinkers(
		"namespace",
		"node",
		"persistentvolume",
		"storageclass",
	)

	probe.AppendNamespaceLinkers(
		"configmap",
		"cronjob",
		"deployment",
		"daemonset",
		"endpoints",
		"ingress",
		"job",
		"networkpolicy",
		"pod",
		"persistentvolumeclaim",
		"replicaset",
		"replicationcontroller",
		"secret",
		"service",
		"statefulset",
	)

	return probe, nil
}
