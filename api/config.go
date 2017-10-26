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

type configAPI struct {
	cfg *viper.Viper
}

func normalizeValue(v interface{}) interface{} {
	switch v := v.(type) {
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(v))
		for key, value := range v {
			key := key.(string)
			value = normalizeValue(value)
			m[key] = value
		}
		return m
	case []interface{}:
		for i, value := range v {
			v[i] = normalizeValue(value)
		}
	}
	return v
}

func (c *configAPI) configGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	vars := mux.Vars(&r.Request)
	key := vars["key"]
	value := normalizeValue(c.cfg.Get(key))
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		panic(err)
	}
}

func (c *configAPI) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			Name:        "ConfigGet",
			Method:      "GET",
			Path:        "/api/config/{key}",
			HandlerFunc: c.configGet,
		},
	}

	r.RegisterRoutes(routes)
}

// RegisterConfigAPI registers a configuration endpoint (read only) in API server
func RegisterConfigAPI(r *shttp.Server) {
	c := &configAPI{
		cfg: config.GetConfig(),
	}

	c.registerEndpoints(r)
}
