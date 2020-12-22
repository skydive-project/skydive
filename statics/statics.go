/*
 * Copyright (C) 2020 Sylvain Baubeau
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

package statics

import "github.com/skydive-project/skydive/graffiti/js"

// Assets implements the asset provider interface
type assets struct{}

// Asset returns the content of a bundled file
func (a *assets) Asset(name string) ([]byte, error) {
	if name == "bootstrap.js" {
		// Load Skydive API
		api, err := Asset("js/api.js")
		if err != nil {
			return nil, err
		}

		bootstrap, err := js.Asset("bootstrap.js")
		if err != nil {
			return nil, err
		}

		return append(api, bootstrap...), nil
	}

	// Allow override Graffiti asset
	content, err := Asset(name)
	if err == nil {
		return content, nil
	}

	// Fallback to Graffiti asset
	return js.Asset(name)
}

// Assets is a asset provider singleton
var Assets assets
