package server

import (
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
	"github.com/skydive-project/skydive/logging"
	"github.com/skydive-project/skydive/rbac"
)

// StatusReporter is the interface to report the status of a service
type StatusReporter interface {
	GetStatus() interface{}
}

type statusAPI struct {
	reporter StatusReporter
}

func (s *statusAPI) statusGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	if !rbac.Enforce(r.Username, "status", "read") {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	status := s.reporter.GetStatus()
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		logging.GetLogger().Warningf("Error while writing response: %s", err)
	}
}

func (s *statusAPI) registerEndpoints(r *shttp.Server, authBackend shttp.AuthenticationBackend) {
	routes := []shttp.Route{
		{
			Name:        "StatusGet",
			Method:      "GET",
			Path:        "/api/status",
			HandlerFunc: s.statusGet,
		},
	}

	r.RegisterRoutes(routes, authBackend)
}

// RegisterStatusAPI registers the status API endpoint
func RegisterStatusAPI(s *shttp.Server, r StatusReporter, authBackend shttp.AuthenticationBackend) {
	a := &statusAPI{
		reporter: r,
	}

	a.registerEndpoints(s, authBackend)
}
