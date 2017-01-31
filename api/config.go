package api

import (
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	"github.com/gorilla/mux"

	"github.com/skydive-project/skydive/config"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/spf13/viper"
)

type ConfigApi struct {
	cfg *viper.Viper
}

func (c *ConfigApi) configGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(&r.Request)
	// lookup into ConfigApi
	Value := c.cfg.GetString(vars["key"])
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(Value); err != nil {
		panic(err)
	}
}

func (c *ConfigApi) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			"ConfigGet",
			"GET",
			"/api/config/{key}",
			c.configGet,
		},
	}

	r.RegisterRoutes(routes)
}

func RegisterConfigApi(r *shttp.Server) {
	c := &ConfigApi{
		cfg: config.GetConfig(),
	}

	c.registerEndpoints(r)
}
