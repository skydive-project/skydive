/*
 * Copyright 2016 IBM Corp.
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

package probes

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/topology/graph"

	"github.com/FlorianOtel/client-go/kubernetes"
	"github.com/FlorianOtel/client-go/pkg/api/v1"
	"github.com/FlorianOtel/client-go/pkg/types"
	"github.com/FlorianOtel/client-go/tools/clientcmd"
)

const (
	NotEqual       = 0
	DirectionEqual = 1
	AllEqual       = 2
)

const uniDirectional = "->"

type k8sNP struct {
	from      graph.Identifier
	to        graph.Identifier
	m         graph.Metadata
	fromStr   string // for sorting
	toStr     string // for sorting
	direction string
	npText    string
	e         graph.Identifier
}

type K8SCtlMessage struct {
	msgType string
	obj     interface{}
}

type k8sPodEdge struct {
	Name      string
	GraphNode graph.Identifier // skydive
}

type k8sPodsStruct struct {
	Name      string
	GraphNode graph.Identifier // skydive
	Namespace string
	UID       types.UID // k8s
	Labels    map[string]string
	Hostname  string
}

type K8SCtlProbe struct {
	Graph     *graph.Graph
	NetnsIDs  map[interface{}]graph.Identifier
	k8sPods   map[string]k8sPodsStruct
	k8sNPNEW  []k8sNP
	k8sNPOLD  []k8sNP
	clientset *kubernetes.Clientset
	maxNP     int
	t         time.Duration
}

func valueToStringGenerated(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}

func (k8s *K8SCtlProbe) updatek8sNP() {
	for {
		k8s.k8sNPOLD = make([]k8sNP, k8s.maxNP)
		k8s.k8sNPOLD = k8s.k8sNPNEW
		k8s.k8sNPNEW = make([]k8sNP, k8s.maxNP)

		k8s.getk8sPods()
		k8s.syncWithK8s()
		k8s.getk8sNP()
		k8s.copyEdgeIds()
		k8s.updateNPLinks()
		time.Sleep(k8s.t)
	}
}

// Copy corresponding Edge ID to each Link (Edge ID is assigned to k8s.k8sNPNEW by k8s.updateNPLinks() only when the Edge is first added
// Need to preserve it when updating k8s.k8sNPOLD/NEW ...
func (k8s *K8SCtlProbe) copyEdgeIds() {
	for i, link := range k8s.k8sNPNEW {
		if link.from == "" || link.to == "" {
			break
		}
		edge := k8s.findEdge(link)
		if edge != "" {
			k8s.k8sNPNEW[i].e = edge
		}
	}
}

// retrun Edge ID of NP link is exist
func (k8s *K8SCtlProbe) findEdge(link k8sNP) graph.Identifier {
	for _, linkOLD := range k8s.k8sNPOLD {
		if linkOLD.from == link.from && linkOLD.to == link.to {
			return linkOLD.e
		}
	}
	return ""
}

// retrun the skydive ID of the matched pod according to given namespace
func (k8s *K8SCtlProbe) findPodByNS(ns string) (bool, []k8sPodEdge) {
	var ret []k8sPodEdge
	var valid = false
	for name, data := range k8s.k8sPods {
		if data.Namespace == ns {
			var pod k8sPodEdge
			pod.GraphNode = data.GraphNode
			pod.Name = name
			ret = append(ret, pod)
			valid = true
		}
	}
	return valid, ret
}

// retrun the skydive ID of the matched pod according to given label key/value
func (k8s *K8SCtlProbe) findPodByLabel(key string, value string, ns string) (bool, k8sPodEdge) {
	var ret k8sPodEdge
	for name, data := range k8s.k8sPods {
		labels := data.Labels
		if labels[key] == value {
			if ns != "" {
				if data.Namespace == ns {
					ret.GraphNode = data.GraphNode
					ret.Name = name
					return true, ret
				}
			} else {
				ret.GraphNode = data.GraphNode
				ret.Name = name
				return true, ret
			}
		}
	}
	return false, ret
}

// sync between pods (k8s.k8sPods) as reported by k8s and skydive netns IDs (k8s.NetnsIDs)
func (k8s *K8SCtlProbe) syncWithK8s() {
	for _, netns := range k8s.Graph.GetNodes(graph.Metadata{"Type": "netns"}) {
		for name, data := range k8s.k8sPods {
			k8s.Graph.RLock()
			netnsName, err := netns.GetFieldString("PodName")
			k8s.Graph.RUnlock()
			if err != nil {
				logging.GetLogger().Debugf("Can't find Name field of netns ID %s", netns.ID)
				continue
			}
			if strings.Contains(netnsName, string(data.UID)) {
				// update k8sPods
				data.GraphNode = netns.ID
				k8s.k8sPods[name] = data
				logging.GetLogger().Debugf("Update pod %s with ID %s", name, data.GraphNode)
			}
		}
	}
}

// gets pods from k8s
func (k8s *K8SCtlProbe) getk8sPods() bool {
	pods, err := k8s.clientset.Core().Pods("").List(v1.ListOptions{})
	if err != nil {
		logging.GetLogger().Error(err)
	}
	logging.GetLogger().Debugf("There are %d pods in the cluster", len(pods.Items))
	if len(pods.Items) == 0 {
		logging.GetLogger().Debugf("There are no k8s pods - skipping")
		return false
	}

	for _, pod := range pods.Items {
		logging.GetLogger().Debugf("%s %s %s", pod.GetName(), pod.GetCreationTimestamp(), pod.GetUID())
		if val, ok := k8s.k8sPods[pod.GetName()]; ok {
			// update
			val.Namespace = pod.GetNamespace()
			val.UID = pod.GetUID()
			val.Labels = pod.GetLabels()
			val.Hostname = pod.Spec.Hostname
			k8s.k8sPods[pod.GetName()] = val
		} else {
			// new
			val := k8sPodsStruct{Namespace: pod.GetNamespace(), UID: pod.GetUID(), Labels: pod.GetLabels(), Hostname: pod.Spec.Hostname, GraphNode: ""}
			k8s.k8sPods[pod.GetName()] = val
		}
	}
	logging.GetLogger().Debugf("k8s.k8sPods: %s", k8s.k8sPods)
	return true
}

// Update NP links (add/remove)
func (k8s *K8SCtlProbe) updateNPLinks() {
	sort.Slice(k8s.k8sNPNEW, func(i, j int) bool {
		return strings.Join([]string{k8s.k8sNPNEW[i].fromStr, k8s.k8sNPNEW[i].toStr}, "") > strings.Join([]string{k8s.k8sNPNEW[j].fromStr, k8s.k8sNPNEW[j].toStr}, "")
	})

	deepequal := reflect.DeepEqual(k8s.k8sNPOLD, k8s.k8sNPNEW)
	logging.GetLogger().Debugf("Is equal (OLD/NEW) = %v", deepequal)

	if deepequal {
		return
	}

	var old int
	var new int
	logging.GetLogger().Debugf("k8sctl probe: OLD %v", k8s.k8sNPOLD)
	logging.GetLogger().Debugf("k8sctl probe: NEW %v", k8s.k8sNPNEW)

	k8s.Graph.Lock()

	for !(k8s.k8sNPOLD[old].from == "" && k8s.k8sNPNEW[new].from == "") {
		logging.GetLogger().Debugf("k8s.k8sNPOLD[old].from = %s \n k8s.k8sNPOLD[old].to = %s \n k8s.k8sNPNEW[new].from = %s \n k8s.k8sNPNEW[new].to = %s", k8s.k8sNPOLD[old].from, k8s.k8sNPOLD[old].to, k8s.k8sNPNEW[new].from, k8s.k8sNPNEW[new].to)

		if k8s.k8sNPOLD[old].from == "" && k8s.k8sNPNEW[new].from != "" && k8s.k8sNPNEW[new].to != "" {
			// add new
			ID := k8s.addNPEdge(k8s.k8sNPNEW[new])
			k8s.k8sNPNEW[new].e = ID
			new++

		} else if k8s.k8sNPOLD[old].from != "" && k8s.k8sNPOLD[old].to != "" && k8s.k8sNPNEW[new].from == "" {
			// remove old
			logging.GetLogger().Debugf("Remove Link %s to %s %s (%s)", k8s.k8sNPOLD[old].fromStr, k8s.k8sNPOLD[old].toStr, k8s.k8sNPOLD[old].direction, k8s.k8sNPOLD[old].npText)
			edge := k8s.Graph.GetEdge(k8s.k8sNPOLD[old].e)
			if edge == nil {
				logging.GetLogger().Debugf("Failed to Get Edge ID %s", k8s.k8sNPOLD[old].e)
			} else {
				k8s.Graph.DelEdge(edge)
			}
			old++
		} else {

			l := k8s.comapreNP(k8s.k8sNPOLD[old], k8s.k8sNPNEW[new])

			if l == AllEqual {
				old++
				new++
			} else if l == DirectionEqual {
				// update
				k8s.Graph.SetMetadata(k8s.k8sNPNEW[old].e, k8s.k8sNPNEW[new].m)
				k8s.k8sNPNEW[new].e = k8s.k8sNPNEW[old].e
				new++
				old++
			} else if l == NotEqual {
				if k8s.k8sNPOLD[old].fromStr < k8s.k8sNPNEW[new].fromStr {
					// new to be add
					ID := k8s.addNPEdge(k8s.k8sNPNEW[new])
					k8s.k8sNPNEW[new].e = ID
					new++
				} else {
					// old to be remove
					logging.GetLogger().Debugf("Remove Link %s to %s %s (%s)", k8s.k8sNPOLD[old].fromStr, k8s.k8sNPOLD[old].toStr, k8s.k8sNPOLD[old].direction, k8s.k8sNPOLD[old].npText)
					logging.GetLogger().Debugf("Remove Edge %s", k8s.k8sNPOLD[old].e)
					edge := k8s.Graph.GetEdge(k8s.k8sNPOLD[old].e)
					if edge == nil {
						logging.GetLogger().Debugf("Failed to Get Edge ID %s", k8s.k8sNPOLD[old].e)
					} else {
						k8s.Graph.DelEdge(edge)
					}
					old++
				}
			}
		}
	}
	k8s.Graph.Unlock()
}

// Add Link procedure
func (k8s *K8SCtlProbe) addNPEdge(np k8sNP) graph.Identifier {
	logging.GetLogger().Debugf("Attempt to Add Link %s to %s %s (%s)", np.fromStr, np.toStr, np.direction, np.npText)

	fromNode := k8s.Graph.GetNode(np.from)
	if fromNode == nil {
		logging.GetLogger().Debugf("Failed to Add From Edge Node ID %s", np.from)
		return ""
	}

	toNode := k8s.Graph.GetNode(np.to)
	if toNode == nil {
		logging.GetLogger().Debugf("Failed to Add To Edge Node ID %s", np.to)
		return ""
	}

	link := k8s.Graph.Link(fromNode, toNode, np.m)
	logging.GetLogger().Debugf("Added Edge %s", np.e)
	return link.ID
}

// compare k8s np
func (k8s *K8SCtlProbe) comapreNP(old k8sNP, new k8sNP) int {
	if old.from == new.from && old.to == new.to {
		if old.m["NetworkPolicy"] == new.m["NetworkPolicy"] && old.m["Direction"] == new.m["Direction"] {
			return AllEqual
		}
		return DirectionEqual
	}
	return NotEqual
}

// gets network policies from k8s
func (k8s *K8SCtlProbe) getk8sNP() {
	// get name spaces
	nss, err := k8s.clientset.CoreV1().Namespaces().List(v1.ListOptions{})
	if err != nil {
		logging.GetLogger().Error(err)
	}

	var i int
	for _, ns := range nss.Items {

		var deny = false
		// get ns annotations
		if len(ns.Annotations) > 0 {
			for _, v := range ns.Annotations {
				if strings.Contains(v, "DefaultDeny") {
					logging.GetLogger().Debugf("\t\t DefaultDeny")
					deny = true
				}
			}
		}

		// get network policies
		nps, err := k8s.clientset.ExtensionsV1beta1().NetworkPolicies(ns.GetName()).List(v1.ListOptions{})
		if err != nil {
			logging.GetLogger().Debugf("Error: " + err.Error())
		}
		logging.GetLogger().Debugf("%s ns: There are %d network policies in the cluster", ns.GetName(), len(nps.Items))

		for _, np := range nps.Items {

			var npText string
			var FromPod []k8sPodEdge
			var ToPod []k8sPodEdge
			for ingress := range np.Spec.Ingress {
				// Only single descriptor is used to choose the "From" np
				for from := range np.Spec.Ingress[ingress].From {
					PodSelectorLabels := np.Spec.Ingress[ingress].From[from].PodSelector
					if PodSelectorLabels != nil {

						for key, value := range PodSelectorLabels.MatchLabels {
							valid, id := k8s.findPodByLabel(key, value, "")
							if valid {
								FromPod = append(FromPod, id)
							}
						}
						break
					}

					NamespaceSelectorLabels := np.Spec.Ingress[ingress].From[from].NamespaceSelector
					if NamespaceSelectorLabels != nil {
						for _, value := range NamespaceSelectorLabels.MatchLabels {
							// key/value for ns are actually the namespace name itself
							valid, id := k8s.findPodByNS(value)
							if valid {
								FromPod = id
							}
						}
						break
					}
				}

				if len(np.Spec.Ingress[ingress].Ports) > 0 {

					for port := range np.Spec.Ingress[ingress].Ports {
						ProtocolStr := valueToStringGenerated(np.Spec.Ingress[ingress].Ports[port].Protocol)
						npText = fmt.Sprintf("%s %s/%s ", npText, strings.Replace(ProtocolStr, "*", "", -1), np.Spec.Ingress[ingress].Ports[port].Port)
					}
				}
			}

			if len(np.Spec.PodSelector.MatchLabels) > 0 {
				for key, value := range np.Spec.PodSelector.MatchLabels {
					valid, id := k8s.findPodByLabel(key, value, ns.GetName())
					if valid {
						ToPod = append(ToPod, id)
					}
				}
			} else {
				//choose all pods in the given ns
				valid, pod := k8s.findPodByNS(ns.GetName())
				if valid {
					ToPod = pod
				}
			}

			// build the np edge - skip if not deny - to avoid system ns which allows all-to-all communication between their pods
			if deny {
				for _, fromp := range FromPod {
					for _, top := range ToPod {
						logging.GetLogger().Debugf("fromp.GraphNode = %s  top.GraphNode = %s", fromp.GraphNode, top.GraphNode)
						if (fromp.GraphNode != "") && (top.GraphNode != "") {
							if strings.Compare(npText, "") != 0 {
								npText = fmt.Sprintf(" ingress: %s", npText)
							}

							m := graph.Metadata{"Application": "k8s", "RelationType": "netpolicy", "LinkType": "policy", "NetworkPolicy": npText, "Direction": uniDirectional, "From": fromp.Name, "To": top.Name}

							NS := k8sNP{
								from:      fromp.GraphNode,
								to:        top.GraphNode,
								m:         m,
								fromStr:   fromp.Name,
								toStr:     top.Name,
								direction: uniDirectional,
								npText:    npText,
							}
							k8s.k8sNPNEW[i] = NS
							i = i + 1
							logging.GetLogger().Debugf("New NP has been added to k8s.k8sNPNEW")
						}
					}
				}
			}
		}
	}
}

func (k8s *K8SCtlProbe) Start() {
	logging.GetLogger().Debugf("k8sctl probe: got command - Start")
	go k8s.updatek8sNP()
}

func (k8s *K8SCtlProbe) Stop() {
	logging.GetLogger().Debugf("k8sctl probe: got command - Stop")
}

func NewK8SCtlProbe(g *graph.Graph) (*K8SCtlProbe, error) {
	logging.GetLogger().Debugf("k8sctl probe: got command - NewK8SCtlProbe")
	k8s := &K8SCtlProbe{
		Graph: g,
	}

	k8s.maxNP = config.GetConfig().GetInt("k8s.max_network_policies")
	logging.GetLogger().Debugf("k8sctl probe: k8s.max_network_policies = %s", k8s.maxNP)

	k8s.t = config.GetConfig().GetDuration("k8s.polling_rate")
	if k8s.t.Seconds() >= 1 {
		logging.GetLogger().Infof("Update every %s", k8s.t)
	} else {
		logging.GetLogger().Errorf("k8s.polling_rate (%s) is lower than 1s", k8s.t)
	}

	kubeconfig := config.GetConfig().GetString("k8s.config_file")
	// uses the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("k8sctl probe: %s  (k8s config_file = %s)", err.Error(), kubeconfig)
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8sctl probe: %s", err.Error())
	}
	k8s.clientset = clientset

	k8s.NetnsIDs = make(map[interface{}]graph.Identifier)
	k8s.k8sPods = make(map[string]k8sPodsStruct)

	k8s.k8sNPOLD = make([]k8sNP, k8s.maxNP)
	k8s.k8sNPNEW = make([]k8sNP, k8s.maxNP)
	return k8s, nil
}
