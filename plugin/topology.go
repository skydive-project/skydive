/*
 * Copyright (C) 2019 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package plugin

import (
	"fmt"
	"path"
	"plugin"
	"reflect"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/probe"
	"github.com/skydive-project/skydive/topology/probes"
)

// TopologyPlugin defines topology plugin
type TopologyPlugin struct {
	Name         string
	Register     func()
	AgentCtor    func(ctx probes.Context, bundle *probe.Bundle) (probes.Handler, error)
	AnalyzerCtor func(ctx probes.Context, bundle *probe.Bundle) (probes.Handler, error)
}

// LoadTopologyPlugins load topology plugins
func LoadTopologyPlugins() ([]TopologyPlugin, error) {
	var plugins []TopologyPlugin

	pluginsDir := config.GetString("plugin.plugins_dir")
	probeList := config.GetStringSlice("plugin.topology.probes")

	logging.GetLogger().Infof("Topology plugins: %v", probeList)

	for _, so := range probeList {
		filename := path.Join(pluginsDir, so+".so")
		logging.GetLogger().Infof("Loading plugin %s", filename)

		plugin, err := plugin.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("Failed to load plugin %s: %s", so, err)
		}

		tp := TopologyPlugin{Name: so}

		if symbol, err := plugin.Lookup("Register"); err == nil {
			fnc, ok := symbol.(func())
			if !ok {
				return nil, fmt.Errorf("Invalid plugin %s, %s", so, reflect.TypeOf(symbol))
			}

			tp.Register = fnc
		}

		if symbol, err := plugin.Lookup("NewAgentProbe"); err == nil {
			fnc, ok := symbol.(func(ctx probes.Context, bundle *probe.Bundle) (probes.Handler, error))
			if !ok {
				return nil, fmt.Errorf("Invalid plugin %s, %s", so, reflect.TypeOf(symbol))
			}

			tp.AgentCtor = fnc
		}

		if symbol, err := plugin.Lookup("NewAnalyzerProbe"); err == nil {
			fnc, ok := symbol.(func(ctx probes.Context, bundle *probe.Bundle) (probes.Handler, error))
			if !ok {
				return nil, fmt.Errorf("Invalid plugin %s, %s", so, reflect.TypeOf(symbol))
			}

			tp.AnalyzerCtor = fnc
		}

		if tp.Register == nil {
			return nil, fmt.Errorf("Non compliant plugin '%s': Register function not found", so)
		}

		if tp.AgentCtor == nil && tp.AnalyzerCtor == nil {
			return nil, fmt.Errorf("Non compliant plugin '%s': no constructor found", so)
		}

		plugins = append(plugins, tp)
	}

	return plugins, nil
}
