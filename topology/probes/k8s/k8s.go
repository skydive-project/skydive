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

// K8sProbe defines the k8s probe
type K8sProbe struct {
	*Probe
	clusterSubprobe Subprobe
}

// Start the k8s probe
func (p *K8sProbe) Start() error {
	if err := p.clusterSubprobe.Start(); err != nil {
		return err
	}

	return p.Probe.Start()
}

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
func NewK8sProbe(g *graph.Graph) (*K8sProbe, error) {
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

	clusterName := getClusterName(kubeconfig)

	subprobeHandlers := map[string]SubprobeHandler{
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

	InitSubprobes(enabledSubprobes, subprobeHandlers, clientset, g, Manager, clusterName)

	linkerHandlers := []LinkHandler{
		newContainerDockerLinker,
		newReplicaSetPodLinker,
		newDeploymentReplicaSetLinker,
		newDaemonSetPodLinker,
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

	verifiers := []probe.Handler{}

	probe := &K8sProbe{
		Probe:           NewProbe(g, Manager, subprobes[Manager], linkers, verifiers),
		clusterSubprobe: initClusterSubprobe(g, Manager, clusterName),
	}

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

func getClusterName(kubeconfig *clientcmd.ClientConfig) string {
	clusterName := config.GetString("analyzer.topology.k8s.cluster_name")
	if len(clusterName) == 0 {
		clusterName = "cluster"

		if kubeconfig != nil {
			rawconfig, err := (*kubeconfig).RawConfig()
			if err == nil {
				if context := rawconfig.Contexts[rawconfig.CurrentContext]; context != nil {
					if context.Cluster != "" {
						clusterName = context.Cluster
					}
				}
			}
		}
	}
	return clusterName
}
