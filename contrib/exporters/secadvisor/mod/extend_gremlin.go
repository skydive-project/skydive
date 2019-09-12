/*
 * Copyright (C) 2019 IBM, Inc.
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

package mod

import (
	"bytes"
	"encoding/json"
	"strings"
	"text/template"
	"time"

	cache "github.com/pmylund/go-cache"
	"github.com/skydive-project/skydive/api/client"
	"github.com/skydive-project/skydive/contrib/exporters/core"
	"github.com/skydive-project/skydive/logging"
	"github.com/spf13/viper"
)

// extendGremlin is used for transform extensions using gremlin expressions
// Syntax supported is:
//	VAR_NAME=<gremlin expression with a substitution strings>
// Substitution strings are defined according to golang template usage using {{ and }}
// For example:
//	AA_Name=G.V().Has('RoutingTables.Src','{{.Network.A}}').Values('Host')
// The substitution string refers to fields that exist in the data provided by the particular transform
// If needed, be sure to put quotes around the substitution results
// Use only single quotes in the gremlin expression
// Need to define `Extend` field as a map[string]interface{} subfield of `transform`
// Structure in yml file is as follows:
//   transform:
//     type: sa
//     sa:
//       extend:
//         - VAR_NAME1=<gremlin expression with substitution strings>
//         - VAR_NAME2=<gremlin expression with substitution strings>
type extendGremlin struct {
	gremlinClient    *client.GremlinQueryHelper
	gremlinExprCache *cache.Cache
	gremlinTemplates []*template.Template
	newVars          []string
	nTemplates       int
}

func NewExtendGremlin(cfg *viper.Viper) *extendGremlin {
	gremlinClient := client.NewGremlinQueryHelper(core.CfgAuthOpts(cfg))
	extendStrings := cfg.GetStringSlice(core.CfgRoot + "transform.extend")
	var gremlinTemplates []*template.Template
	var newVars []string
	var nTemplates int
	nTemplates = 0
	gremlinTemplates = make([]*template.Template, len(extendStrings))
	newVars = make([]string, len(extendStrings))
	// parse the extension expressions and save them for future use
	for _, ee := range extendStrings {
		substrings := strings.SplitN(ee, "=", 2)
		vv, err := template.New("extend_template").Parse(substrings[1])
		if err != nil {
			logging.GetLogger().Errorf("NewExtendGremlin: template error: %s", err)
			continue
		}
		gremlinTemplates[nTemplates] = vv
		newVars[nTemplates] = substrings[0]
		nTemplates++
	}
	p := &extendGremlin{
		gremlinClient:    gremlinClient,
		gremlinExprCache: cache.New(10*time.Minute, 10*time.Minute),
		gremlinTemplates: gremlinTemplates,
		newVars:          newVars,
		nTemplates:       nTemplates,
	}
	return p
}

// Extend: main function called to perform the extension substitutions
// we expect the data of a single flow per call
func (e *extendGremlin) Extend(in *SecurityAdvisorFlow) {
	for i := 0; i < e.nTemplates; i++ {
		var tpl bytes.Buffer
		var err error
		// apply the gremlin expression
		err = e.gremlinTemplates[i].Execute(&tpl, in)
		if err != nil {
			logging.GetLogger().Errorf("Extend: gremlin template = %s", e.gremlinTemplates[i])
			logging.GetLogger().Errorf("Extend: template parse error; err = %s", err)
			continue
		}
		result := tpl.String()
		// check if result is already in cache; if not, perform gremlin request
		result2, ok := e.gremlinExprCache.Get(result)
		if !ok {
			// need to perform the gremlin query
			result3, err := e.gremlinClient.Query(result)
			if err != nil {
				logging.GetLogger().Errorf("Extend: gremlin query = %s", result)
				logging.GetLogger().Errorf("Extend: gremlin query error; err = %s", err)
				continue
			}
			if len(result3) == 0 {
				logging.GetLogger().Errorf("Extend: gremlin query error; result = %s", result)
				continue
			}
			var result4 []string
			if err = json.Unmarshal(result3, &result4); err != nil {
				logging.GetLogger().Errorf("Extend: unmarshal error; err = %s", err)
				continue
			}
			// add new field in map with the desired key name
			if 0 == len(result4) {
				continue
			}
			e.gremlinExprCache.Set(result, result4[0], cache.DefaultExpiration)
			result2 = result4[0]
		}
		in.Extend[e.newVars[i]] = result2
	}
}
