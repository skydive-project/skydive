package server

import (
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/config"
	"github.com/skydive-project/skydive/graffiti/graph"
	"github.com/skydive-project/skydive/graffiti/logging"
	"github.com/skydive-project/skydive/graffiti/rbac"
	shttp "github.com/skydive-project/skydive/http"
)

type configAPI struct {
	cfg *config.SkydiveConfig
}

func (c *configAPI) configGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	key := vars["key"]
	if !rbac.Enforce(r.Username, "config", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	value := graph.NormalizeValue(c.cfg.Get(key))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logging.GetLogger().Warningf("Error while writing response: %s", err)
	}
}

func (c *configAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	// swagger:operation GET /config/{key} getConfig
	//
	// Get configuration value
	//
	// ---
	// summary: Get configuration
	//
	// tags:
	// - Config
	//
	// produces:
	// - application/json
	//
	// schemes:
	// - http
	// - https
	//
	// parameters:
	// - name: key
	//   in: path
	//   required: true
	//   type: string
	//
	// responses:
	//   200:
	//     description: configuration value
	//     schema:
	//       $ref: '#/definitions/AnyValue'

	routes := []shttp.Route{
		{
			Name:        "ConfigGet",
			Method:      "GET",
			Path:        "/api/config/{key}",
			HandlerFunc: c.configGet,
		},
	}

	r.RegisterRoutes(routes, authBackend)
}

// RegisterConfigAPI registers a configuration endpoint (read only) in API server
func RegisterConfigAPI(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	c := &configAPI{
		cfg: config.GetConfig(),
	}

	c.registerEndpoints(r, authBackend)
}
