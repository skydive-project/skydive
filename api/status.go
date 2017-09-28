package api

import (
	"encoding/json"
	"net/http"

	auth "github.com/abbot/go-http-auth"
	shttp "github.com/skydive-project/skydive/http"
)

// WSSpeaker is the interface to report the status of a service
type StatusReporter interface {
	GetStatus() interface{}
}

type statusAPI struct {
	reporter StatusReporter
}

func (s *statusAPI) statusGet(w http.ResponseWriter, r *auth.AuthenticatedRequest) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	status := s.reporter.GetStatus()
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		panic(err)
	}
}

func (s *statusAPI) registerEndpoints(r *shttp.Server) {
	routes := []shttp.Route{
		{
			Name:        "StatusGet",
			Method:      "GET",
			Path:        "/api/status",
			HandlerFunc: s.statusGet,
		},
	}

	r.RegisterRoutes(routes)
}

// RegisterStatus registers the status endpoint
func RegisterStatusAPI(s *shttp.Server, r StatusReporter) {
	a := &statusAPI{
		reporter: r,
	}

	a.registerEndpoints(s)
}
