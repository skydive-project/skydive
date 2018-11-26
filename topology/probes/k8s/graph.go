/*
 * Copyright 2017 IBM Corp.
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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/filters"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/probe"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Manager is the manager value for Kubernetes
	Manager = "k8s"
	// KubeKey is the metadata area for k8s specific fields
	KubeKey = "K8s"
	// ExtraKey is the metadata area for k8s extra fields
	ExtraKey = "K8s.Extra"
)

// DereferenceValue dereference pointers returning the real value
func DereferenceValue(val reflect.Value) reflect.Value {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	return val
}

// GenericGetFieldKeys underlaying implementation of Getter.GetFieldKeys method
func GenericGetFieldKeys(i interface{}) (keys []string) {
	parent := DereferenceValue(reflect.ValueOf(i))
	for i := 0; i < parent.NumField(); i++ {
		child := DereferenceValue(parent.Field(i))
		typeOfChild := parent.Type().Field(i)
		switch typeOfChild.Name {
		case "MetadataInner":
			keys = append(keys, GenericGetFieldKeys(child.Interface())...)
		default:
			if _, ok := typeOfChild.Tag.Lookup("skydive"); ok {
				keys = append(keys, typeOfChild.Name)
			}
		}
	}
	return
}

// GenericGetFieldTyped underlaying implementation of Getter.GetField<XXX> methods
func GenericGetFieldTyped(i interface{}, key string, ty ...string) (reflect.Value, error) {
	parent := DereferenceValue(reflect.ValueOf(i))
	for i := 0; i < parent.NumField(); i++ {
		child := DereferenceValue(parent.Field(i))
		typeOfChild := parent.Type().Field(i)
		switch typeOfChild.Name {
		case "MetadataInner":
			if grandchild, err := GenericGetFieldTyped(child.Interface(), key, ty...); err != common.ErrFieldNotFound {
				return grandchild, err
			}
		case key:
			if tag, ok := typeOfChild.Tag.Lookup("skydive"); ok {
				for _, v := range strings.Split(tag, ",") {
					if len(ty) == 0 || v == ty[0] {
						return child, nil
					}
				}
				return reflect.Value{}, common.ErrFieldWrongType
			}
		}
	}
	return reflect.Value{}, common.ErrFieldNotFound
}

// GenericGetField underlaying implementation of Getter.GetField method
func GenericGetField(i interface{}, key string) (interface{}, error) {
	val, err := GenericGetFieldTyped(i, key)
	if err != nil {
		return 0, err
	}
	return val.Interface(), nil
}

// GenericGetFieldInt64 underlaying implementation of Getter.GetFieldInt64 method
func GenericGetFieldInt64(i interface{}, key string) (int64, error) {
	val, err := GenericGetFieldTyped(i, key, "int")
	if err != nil {
		return 0, err
	}
	return val.Int(), nil
}

// GenericGetFieldString underlaying implementation of Getter.GetFieldString method
func GenericGetFieldString(i interface{}, key string) (string, error) {
	val, err := GenericGetFieldTyped(i, key, "string")
	if err != nil {
		return "", err
	}
	return val.String(), nil
}

// GenericMetadataDecoder implements a generic json message raw decoder
func GenericMetadataDecoder(inner common.Getter, raw json.RawMessage) (common.Getter, error) {
	if err := json.Unmarshal(raw, inner); err != nil {
		return nil, fmt.Errorf("unable to unmarshal routing table %s: %s", string(raw), err)
	}

	return inner, nil
}

// RegisterNodeDecoder register graph decoder
func RegisterNodeDecoder(decoder graph.MetadataDecoder, ty string) {
	graph.RegisterNodeDecoder(Manager, decoder, ty)
}

// Setup standard type specific fields
func (inner *MetadataInner) Setup(meta *metav1.ObjectMeta, extra interface{}) *MetadataInner {
	inner.Namespace = meta.Namespace
	inner.Name = meta.Name
	inner.Extra = extra
	return inner
}

// MetadataInner contains type specific fields
// easyjson:json
type MetadataInner struct {
	Namespace string `skydive:"string"`
	Name      string `skydive:"string"`
	Extra     interface{}
}

// GetField implements Getter interface
func (inner *MetadataInner) GetField(key string) (interface{}, error) {
	return GenericGetField(inner, key)
}

// GetFieldInt64 implements Getter interface
func (inner *MetadataInner) GetFieldInt64(key string) (int64, error) {
	return GenericGetFieldInt64(inner, key)
}

// GetFieldString implements Getter interface
func (inner *MetadataInner) GetFieldString(key string) (string, error) {
	return GenericGetFieldString(inner, key)
}

// GetFieldKeys implements Getter interface
func (inner *MetadataInner) GetFieldKeys() []string {
	return GenericGetFieldKeys(inner)
}

// MetadataInnerDecoder implements a json message raw decoder
func MetadataInnerDecoder(raw json.RawMessage) (common.Getter, error) {
	var inner MetadataInner
	return GenericMetadataDecoder(&inner, raw)
}

// MetadataField is generates full path of a k8s specific field
func MetadataField(field string) string {
	return KubeKey + "." + field
}

// MetadataFields generates full path of a list of k8s specific fields
func MetadataFields(fields ...string) []string {
	kubeFields := make([]string, len(fields))
	for i, field := range fields {
		kubeFields[i] = MetadataField(field)
	}
	return kubeFields
}

// NewMetadataFields creates internal k8s node metadata struct
func NewMetadataFields(o metav1.Object) graph.Metadata {
	m := graph.Metadata{}
	m["Name"] = o.GetName()
	m["Namespace"] = o.GetNamespace()
	m["Labels"] = o.GetLabels()
	return m
}

// NewMetadata creates a k8s node base metadata struct
func NewMetadata(manager, ty, name string, inner interface{}) graph.Metadata {
	return graph.Metadata{
		"Manager": manager,
		"Type":    ty,
		"Name":    name,
		KubeKey:   inner,
	}
}

// SetState field of node metadata
func SetState(m *graph.Metadata, isUp bool) {
	state := "DOWN"
	if isUp {
		state = "UP"
	}
	m.SetField("State", state)
}

// NewEdgeMetadata creates a new edge metadata
func NewEdgeMetadata(manager, name string) graph.Metadata {
	m := graph.Metadata{
		"Manager":      manager,
		"RelationType": name,
	}
	return m
}

func newTypesFilter(manager string, types ...string) *filters.Filter {
	filtersArray := make([]*filters.Filter, len(types))
	for i, ty := range types {
		filtersArray[i] = filters.NewTermStringFilter("Type", ty)
	}
	return filters.NewAndFilter(
		filters.NewTermStringFilter("Manager", manager),
		filters.NewOrFilter(filtersArray...),
	)
}

func newObjectIndexerFromFilter(g *graph.Graph, h graph.ListenerHandler, filter *filters.Filter, indexes ...string) *graph.MetadataIndexer {
	filtersArray := make([]*filters.Filter, len(indexes)+1)
	filtersArray[0] = filter
	for i, index := range indexes {
		filtersArray[i+1] = filters.NewNotNullFilter(index)
	}
	m := graph.NewElementFilter(filters.NewAndFilter(filtersArray...))
	return graph.NewMetadataIndexer(g, h, m, indexes...)
}

func newResourceLinker(g *graph.Graph, subprobes map[string]Subprobe, srcType string, srcAttrs []string, dstType string, dstAttrs []string, edgeMetadata graph.Metadata) probe.Probe {
	srcCache := subprobes[srcType]
	dstCache := subprobes[dstType]
	if srcCache == nil || dstCache == nil {
		return nil
	}

	srcIndexer := graph.NewMetadataIndexer(g, srcCache, graph.Metadata{"Type": srcType}, srcAttrs...)
	srcIndexer.Start()

	dstIndexer := graph.NewMetadataIndexer(g, dstCache, graph.Metadata{"Type": dstType}, dstAttrs...)
	dstIndexer.Start()

	ml := graph.NewMetadataIndexerLinker(g, srcIndexer, dstIndexer, edgeMetadata)

	linker := &Linker{
		ResourceLinker: ml.ResourceLinker,
	}
	ml.AddEventListener(linker)

	return linker
}

func objectToNode(g *graph.Graph, object metav1.Object) (node *graph.Node) {
	return g.GetNode(graph.Identifier(object.GetUID()))
}

func objectsToNodes(g *graph.Graph, objects []metav1.Object) (nodes []*graph.Node) {
	for _, obj := range objects {
		if node := objectToNode(g, obj); node != nil {
			nodes = append(nodes, node)
		}
	}
	return
}
