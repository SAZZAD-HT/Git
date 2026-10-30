package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"anvil/internal/logging"
	"anvil/internal/version"
)

// Deps are the services the API handlers call. It grows phase by phase (users in P2,
// repos in P3, …); every field is set by `anvil serve`.
type Deps struct {
	// GitVersion is the version of the git binary found at startup, e.g. "2.53.0".
	GitVersion string
}

// Mount registers every /api/v1 route on r. r is already scoped to /api/v1.
func Mount(r chi.Router, d Deps) {
	r.Get("/version", getVersion(d))
}

// versionResponse is the body of GET /api/v1/version.
type versionResponse struct {
	version.Info
	GitVersion string `json:"git_version"`
}

// getVersion serves GET /api/v1/version. It needs no authentication.
func getVersion(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, r, http.StatusOK, versionResponse{Info: version.Get(), GitVersion: d.GitVersion})
	}
}

// writeJSON writes v as a JSON response with the given status.
func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		logging.FromContext(r.Context()).Warn("write response", "err", err)
	}
}
