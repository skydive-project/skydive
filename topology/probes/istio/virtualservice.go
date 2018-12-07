/*
 * Copyright (C) 2018 IBM, Inc.
 *
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 *
 */

package istio

import (
	"fmt"

	kiali "github.com/kiali/kiali/kubernetes"
	"github.com/mitchellh/mapstructure"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes/k8s"
	"k8s.io/api/core/v1"
)

type virtualServiceHandler struct {
}

// Map graph node to k8s resource
func (h *virtualServiceHandler) Map(obj interface{}) (graph.Identifier, graph.Metadata) {
	vs := obj.(*kiali.VirtualService)
	m := k8s.NewMetadataFields(&vs.ObjectMeta)
	return graph.Identifier(vs.GetUID()), k8s.NewMetadata(Manager, "virtualservice", m, vs, vs.Name)
}

// Dump k8s resource
func (h *virtualServiceHandler) Dump(obj interface{}) string {
	vs := obj.(*kiali.VirtualService)
	return fmt.Sprintf("virtualservice{Namespace: %s, Name: %s}", vs.Namespace, vs.Name)
}

func newVirtualServiceProbe(client interface{}, g *graph.Graph) k8s.Subprobe {
	return k8s.NewResourceCache(client.(*kiali.IstioClient).GetIstioNetworkingApi(), &kiali.VirtualService{}, "virtualservices", g, &virtualServiceHandler{})
}

type virtualServicePodLinker struct {
	graph      *graph.Graph
	vsCache    *k8s.ResourceCache
	podCache   *k8s.ResourceCache
	vsIndexer  *graph.Indexer
	podIndexer *graph.Indexer
}

type virtualServiceInfo struct {
	App     string `mapstructure:"host"`
	Version string `mapstructure:"subset"`
}

func (i *virtualServiceInfo) getIndexerValues() []interface{} {
	values := []interface{}{i.App}
	if i.Version != "" {
		values = append(values, i.Version)
	}
	return values
}

type virtualServiceSpec struct {
	HTTP []struct {
		Route []struct {
			Destination virtualServiceInfo `mapstructure:"destination"`
		} `mapstructure:"route"`
	} `mapstructure:"http"`
}

func getVirtualServiceInfos(vs *kiali.VirtualService) ([]virtualServiceInfo, error) {
	vsSpec := &virtualServiceSpec{}
	if err := mapstructure.Decode(vs.Spec, vsSpec); err != nil {
		return nil, err
	}
	var infos []virtualServiceInfo
	for _, http := range vsSpec.HTTP {
		for _, route := range http.Route {
			infos = append(infos, route.Destination)
		}
	}
	return infos, nil
}

func (vspl *virtualServicePodLinker) GetABLinks(vsNode *graph.Node) (edges []*graph.Edge) {
	vs := vspl.vsCache.GetByNode(vsNode)
	if vs, ok := vs.(*kiali.VirtualService); ok {
		infos, _ := getVirtualServiceInfos(vs)
		for _, info := range infos {
			podNodes, _ := vspl.podIndexer.Get(info.getIndexerValues()...)
			for _, podNode := range podNodes {
				id := graph.GenID(string(vsNode.ID), string(podNode.ID), "RelationType", "virtualservice")
				edges = append(edges, vspl.graph.NewEdge(id, vsNode, podNode, nil, ""))
			}
		}
	}
	return
}

func (vspl *virtualServicePodLinker) GetBALinks(podNode *graph.Node) (edges []*graph.Edge) {
	pod := vspl.podCache.GetByNode(podNode)
	if pod, ok := pod.(*v1.Pod); ok {
		infoWithVersion := virtualServiceInfo{App: pod.Labels["app"], Version: pod.Labels["version"]}
		infoWithoutVersion := virtualServiceInfo{App: pod.Labels["app"], Version: ""}
		vsWithVersionNodes, _ := vspl.vsIndexer.Get(infoWithVersion.getIndexerValues()...)
		vsWithoutVersionNodes, _ := vspl.vsIndexer.Get(infoWithoutVersion.getIndexerValues()...)
		vsNodes := append(vsWithVersionNodes, vsWithoutVersionNodes...)
		for _, vsNode := range vsNodes {
			id := graph.GenID(string(vsNode.ID), string(podNode.ID), "RelationType", "virtualservice")
			edges = append(edges, vspl.graph.NewEdge(id, vsNode, podNode, nil, ""))
		}
	}
	return
}

func newVirtualServicePodLinker(g *graph.Graph) probe.Probe {
	vsProbe := k8s.GetSubprobe(Manager, "virtualservice")
	podProbe := k8s.GetSubprobe("k8s", "pod")
	if vsProbe == nil || podProbe == nil {
		return nil
	}
	podCache := podProbe.(*k8s.ResourceCache)
	vsCache := vsProbe.(*k8s.ResourceCache)

	podIndexer := graph.NewIndexer(g, podCache, func(n *graph.Node) (kv map[string]interface{}) {
		if pod := podCache.GetByNode(n); pod != nil {
			pod := pod.(*v1.Pod)
			app, version := pod.Labels["app"], pod.Labels["version"]
			return map[string]interface{}{
				graph.Hash(app, version): pod,
				graph.Hash(app):          pod,
			}
		}
		return
	}, false)
	podIndexer.Start()

	vsIndexer := graph.NewIndexer(g, vsCache, func(n *graph.Node) (kv map[string]interface{}) {
		if vs := vsCache.GetByNode(n); vs != nil {
			vs := vs.(*kiali.VirtualService)
			infos, _ := getVirtualServiceInfos(vs)
			kv = make(map[string]interface{}, len(infos)*2)
			for _, info := range infos {
				kv[graph.Hash(info.App, info.Version)] = vs
				kv[graph.Hash(info.App)] = vs
			}
		}
		return kv
	}, false)
	vsIndexer.Start()

	nvspl := graph.NewResourceLinker(
		g,
		[]graph.ListenerHandler{vsProbe},
		[]graph.ListenerHandler{podProbe},
		&virtualServicePodLinker{
			graph:      g,
			vsCache:    vsCache,
			podCache:   podCache,
			vsIndexer:  vsIndexer,
			podIndexer: podIndexer,
		},
		k8s.NewEdgeMetadata("istio", "virtualservice"),
	)
	return nvspl
}
