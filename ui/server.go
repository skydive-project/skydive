/*
 * Copyright (C) 2018 Red Hat, Inc.
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

package ui

import (
	"html/template"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	auth "github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/rbac"
	"github.com/skydive-project/skydive/statics"
)

// ExtraAssetPrefix is used for extra assets
const ExtraAssetPrefix = "/extra-statics"

// ExtraAsset describes an extra asset to by exported by the server
type ExtraAsset struct {
	Filename string
	Ext      string
	Content  []byte
}

// Server describes the HTTP server for the Skydive UI
// Extra assets to be served by the server can be specified
// Global var is a map of variables that will be used
// when processing Golang templates for the page
type Server struct {
	sync.RWMutex
	httpServer  *shttp.Server
	extraAssets map[string]ExtraAsset
	globalVars  map[string]interface{}
}

// AddGlobalVar adds a global variable with the provided name and value
func (s *Server) AddGlobalVar(key string, v interface{}) {
	s.Lock()
	s.globalVars[key] = v
	s.Unlock()
}

func (s *Server) loadExtraAssets(folder, prefix string) {
	files := []string{}

	err := filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		files = append(files, strings.TrimPrefix(path, folder))
		return nil
	})

	if err != nil {
		logging.GetLogger().Errorf("Unable to load extra assets from %s: %s", folder, err)
		return
	}

	for _, file := range files {
		path := filepath.Join(folder, file)

		data, err := ioutil.ReadFile(path)
		if err != nil {
			logging.GetLogger().Errorf("Unable to load extra asset %s: %s", path, err)
			return
		}

		ext := filepath.Ext(path)

		key := strings.TrimPrefix(filepath.Join(prefix, file), "/")
		logging.GetLogger().Debugf("Added extra static assert: %s", key)
		s.extraAssets[key] = ExtraAsset{
			Filename: filepath.Join(prefix, file),
			Ext:      ext,
			Content:  data,
		}
	}
}

func (s *Server) readStatics(upath string) (content []byte, err error) {
	if asset, ok := s.extraAssets[upath]; ok {
		logging.GetLogger().Debugf("Fetch disk asset: %s", upath)
		content = asset.Content
	} else if content, err = statics.Asset(upath); err != nil {
		logging.GetLogger().Debugf("Fetch embedded asset: %s", upath)
	}
	return
}

func (s *Server) serveStatics(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if strings.HasPrefix(upath, "/") {
		upath = strings.TrimPrefix(upath, "/")
	}

	content, err := s.readStatics(upath)

	if err != nil {
		logging.GetLogger().Errorf("Unable to find the asset %s", upath)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	ext := filepath.Ext(upath)
	ct := mime.TypeByExtension(ext)

	shttp.SetTLSHeader(w, r)
	w.Header().Set("Content-Type", ct+"; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	w.Write(content)
}

// ServeIndex servers the index page
func (s *Server) ServeIndex(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	html, err := s.readStatics("statics/index.html")
	if err != nil {
		logging.GetLogger().Error("Unable to find the asset index.html")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	username := r.Username
	if username == "" {
		username = "admin"
	}

	s.RLock()
	defer s.RUnlock()

	data := struct {
		ExtraAssets map[string]ExtraAsset
		GlobalVars  interface{}
		Permissions []rbac.Permission
	}{
		ExtraAssets: s.extraAssets,
		GlobalVars:  s.globalVars,
		Permissions: rbac.GetPermissionsForUser(username),
	}

	shttp.SetTLSHeader(w, &r.Request)
	w.Header().Set("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)

	tmpl := template.Must(template.New("index").Delims("<<", ">>").Parse(string(html)))
	if err := tmpl.Execute(w, data); err != nil {
		logging.GetLogger().Criticalf("Unable to execute index template: %s", err)
	}
}

// NewServer returns a new Web server that serves the Skydive UI
func NewServer(server *shttp.Server, assetsFolder string) *Server {
	router := server.Router
	s := &Server{
		extraAssets: make(map[string]ExtraAsset),
		globalVars:  make(map[string]interface{}),
		httpServer:  server,
	}

	if assetsFolder != "" {
		s.loadExtraAssets(assetsFolder, ExtraAssetPrefix)
	}

	router.PathPrefix("/statics").HandlerFunc(s.serveStatics)
	router.PathPrefix(ExtraAssetPrefix).HandlerFunc(s.serveStatics)
	router.HandleFunc("/", shttp.NoAuthenticationWrap(s.ServeIndex))

	// server index for the following url as the client side will redirect
	// the user to the correct page
	routes := []shttp.Route{
		{Path: "/topology", Method: "GET", HandlerFunc: s.ServeIndex},
		{Path: "/preference", Method: "GET", HandlerFunc: s.ServeIndex},
		{Path: "/status", Method: "GET", HandlerFunc: s.ServeIndex},
	}
	server.RegisterRoutes(routes, shttp.NewNoAuthenticationBackend())

	return s
}
