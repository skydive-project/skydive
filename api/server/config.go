package server

import (
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/common"
	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
	"github.com/spf13/viper"
)

type configAPI struct {
	cfg *viper.Viper
}

func (c *configAPI) configGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	vars := mux.Vars(&r.Request)
	key := vars["key"]
	if !rbac.Enforce(r.Username, "config", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	value := common.NormalizeValue(c.cfg.Get(key))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logging.GetLogger().Warningf("Error while writing response: %s", err)
	}
}

func (c *configAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
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
