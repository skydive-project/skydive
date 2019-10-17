// +build tools

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

package main

// This is just insane. Go modules are seriously flawed

import (
	_ "git.fd.io/govpp.git/cmd/binapi-generator"
	_ "github.com/aktau/github-release"
	_ "github.com/davecgh/go-spew/spew"
	_ "github.com/go-swagger/go-swagger/cmd/swagger"
	_ "github.com/gogo/protobuf/protoc-gen-gogofaster"
	_ "github.com/golang/protobuf/protoc-gen-go"
	_ "github.com/golangci/golangci-lint"
	_ "github.com/gomatic/renderizer"
	_ "github.com/jteeuwen/go-bindata"
	_ "github.com/mailru/easyjson/easyjson"
	_ "github.com/mattn/goveralls"
	_ "github.com/t-yuki/gocover-cobertura"
	_ "golang.org/x/tools/go/loader"
)
